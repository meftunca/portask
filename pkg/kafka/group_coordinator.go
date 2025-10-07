package kafka

import (
	"fmt"
	"sync"
	"time"
)

// GroupCoordinator manages consumer groups and their states
type GroupCoordinator struct {
	groups          map[string]*ConsumerGroup
	mutex           sync.RWMutex
	rebalancePolicy RebalancePolicy
	sessionTimeout  time.Duration
	rebalanceTimeout time.Duration
}

// Helper constants for group states
const (
	StateEmpty             = ConsumerGroupStateEmpty
	StatePreparingRebalance = ConsumerGroupStatePreparingRebalance
	StateAwaitingSync      = ConsumerGroupStateCompletingRebalance // Use Completing as AwaitingSync
	StateStable            = ConsumerGroupStateStable
	StateDead              = ConsumerGroupStateDead
)

// PartitionAssignment represents a partition assignment for a consumer
type PartitionAssignment struct {
	Topic      string
	Partitions []int32
}

// RebalancePolicy defines how partitions are distributed among consumers
type RebalancePolicy interface {
	Assign(members []string, partitions map[string][]int32) map[string][]PartitionAssignment
}

// RoundRobinRebalancePolicy implements round-robin partition assignment
type RoundRobinRebalancePolicy struct{}

// RangeRebalancePolicy implements range-based partition assignment
type RangeRebalancePolicy struct{}

// NewGroupCoordinator creates a new group coordinator
func NewGroupCoordinator() *GroupCoordinator {
	gc := &GroupCoordinator{
		groups:           make(map[string]*ConsumerGroup),
		rebalancePolicy:  &RoundRobinRebalancePolicy{},
		sessionTimeout:   30 * time.Second,
		rebalanceTimeout: 60 * time.Second,
	}

	// Start heartbeat checker
	go gc.checkHeartbeats()

	return gc
}

// JoinGroup handles a consumer joining a group
func (gc *GroupCoordinator) JoinGroup(
	groupID, memberID, clientID, clientHost, protocolType string,
	sessionTimeout, rebalanceTimeout time.Duration,
	protocols []string, metadata []byte,
) (*JoinGroupResponse, error) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	group := gc.getOrCreateGroup(groupID)
	group.mutex.Lock()
	defer group.mutex.Unlock()

	// If member ID is empty, generate a new one
	if memberID == "" {
		memberID = fmt.Sprintf("%s-%s-%d", clientID, clientHost, time.Now().UnixNano())
	}

	// Check if member already exists
	if existing, exists := group.Members[memberID]; exists {
		// Update existing member
		existing.LastHeartbeat = time.Now()
		existing.SessionTimeout = sessionTimeout
		existing.RebalanceTimeout = rebalanceTimeout
		existing.Metadata = metadata
	} else {
		// Add new member
		member := &GroupMember{
			ID:               memberID,
			ClientID:         clientID,
			ClientHost:       clientHost,
			SessionTimeout:   sessionTimeout,
			RebalanceTimeout: rebalanceTimeout,
			JoinedAt:         time.Now(),
			LastHeartbeat:    time.Now(),
			Metadata:         metadata,
		}

		// First member becomes leader
		if len(group.Members) == 0 {
			group.Leader = memberID
		}

		group.Members[memberID] = member
	}

	// Trigger rebalance if needed
	if group.State == StateStable {
		group.State = StatePreparingRebalance
		group.Generation++
		fmt.Printf("[Kafka] Group %s: Starting rebalance (generation %d)\n", groupID, group.Generation)
	}

	response := &JoinGroupResponse{
		GenerationID: group.Generation,
		ProtocolName: group.Protocol,
		LeaderID:     group.Leader,
		MemberID:     memberID,
		Members:      make([]GroupMemberInfo, 0),
	}

	// If this member is the leader, include all members
	if memberID == group.Leader {
		for id, member := range group.Members {
			response.Members = append(response.Members, GroupMemberInfo{
				MemberID: id,
				Metadata: member.Metadata,
			})
		}
	}

	// Move to awaiting sync state
	group.State = StateAwaitingSync

	return response, nil
}

// SyncGroup handles synchronizing group assignments
func (gc *GroupCoordinator) SyncGroup(
	groupID, memberID string,
	generationID int32,
	assignments map[string][]byte,
) (*SyncGroupResponse, error) {
	gc.mutex.RLock()
	group, exists := gc.groups[groupID]
	gc.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}

	group.mutex.Lock()
	defer group.mutex.Unlock()

	// Validate generation
	if group.Generation != generationID {
		return nil, fmt.Errorf("generation mismatch")
	}

	member, exists := group.Members[memberID]
	if !exists {
		return nil, fmt.Errorf("member not found: %s", memberID)
	}

	// If this is the leader, store assignments for all members
	if memberID == group.Leader && len(assignments) > 0 {
		for id, assignment := range assignments {
			if m, exists := group.Members[id]; exists {
				m.Metadata = assignment // Store as metadata for now
			}
		}
		group.State = StateStable
		group.LastHeartbeat = time.Now()
		fmt.Printf("[Kafka] Group %s: Rebalance complete (generation %d)\n", groupID, generationID)
	}

	response := &SyncGroupResponse{
		Assignment: member.Metadata, // Return metadata as assignment
	}

	return response, nil
}

// Heartbeat handles consumer heartbeats
func (gc *GroupCoordinator) Heartbeat(groupID, memberID string, generationID int32) error {
	gc.mutex.RLock()
	group, exists := gc.groups[groupID]
	gc.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	group.mutex.Lock()
	defer group.mutex.Unlock()

	// Validate generation
	if group.Generation != generationID {
		return fmt.Errorf("generation mismatch: expected %d, got %d", group.Generation, generationID)
	}

	member, exists := group.Members[memberID]
	if !exists {
		return fmt.Errorf("member not found: %s", memberID)
	}

	// Update heartbeat time
	member.LastHeartbeat = time.Now()

	return nil
}

// LeaveGroup handles a consumer leaving a group
func (gc *GroupCoordinator) LeaveGroup(groupID, memberID string) error {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	group, exists := gc.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	group.mutex.Lock()
	defer group.mutex.Unlock()

	delete(group.Members, memberID)

	fmt.Printf("[Kafka] Member %s left group %s\n", memberID, groupID)

	// If group is empty, mark as empty
	if len(group.Members) == 0 {
		group.State = StateEmpty
		group.Leader = ""
		group.Generation = 0
	} else {
		// Trigger rebalance
		group.State = StatePreparingRebalance
		group.Generation++

		// Elect new leader if needed
		if memberID == group.Leader {
			for id := range group.Members {
				group.Leader = id
				break
			}
		}
	}

	return nil
}

// DescribeGroups returns information about consumer groups
func (gc *GroupCoordinator) DescribeGroups(groupIDs []string) map[string]*GroupDescription {
	gc.mutex.RLock()
	defer gc.mutex.RUnlock()

	result := make(map[string]*GroupDescription)

	for _, groupID := range groupIDs {
		if group, exists := gc.groups[groupID]; exists {
			group.mutex.RLock()

			members := make([]MemberDescription, 0, len(group.Members))
			for id, member := range group.Members {
				members = append(members, MemberDescription{
					MemberID:   id,
					ClientID:   member.ClientID,
					ClientHost: member.ClientHost,
					Metadata:   member.Metadata,
					Assignment: member.Metadata,
				})
			}

			result[groupID] = &GroupDescription{
				GroupID:       groupID,
				State:         group.State.String(),
				ProtocolType:  group.Protocol,
				Protocol:      group.Protocol,
				Members:       members,
				Generation:    group.Generation,
			}

			group.mutex.RUnlock()
		}
	}

	return result
}

// ListGroups returns all consumer groups
func (gc *GroupCoordinator) ListGroups() []GroupOverview {
	gc.mutex.RLock()
	defer gc.mutex.RUnlock()

	result := make([]GroupOverview, 0, len(gc.groups))

	for groupID, group := range gc.groups {
		group.mutex.RLock()
		result = append(result, GroupOverview{
			GroupID:      groupID,
			ProtocolType: group.Protocol,
			State:        group.State.String(),
		})
		group.mutex.RUnlock()
	}

	return result
}

// checkHeartbeats periodically checks for expired heartbeats
func (gc *GroupCoordinator) checkHeartbeats() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		gc.mutex.RLock()
		groups := make([]*ConsumerGroup, 0, len(gc.groups))
		for _, group := range gc.groups {
			groups = append(groups, group)
		}
		gc.mutex.RUnlock()

		for _, group := range groups {
			group.mutex.Lock()

			expiredMembers := make([]string, 0)
			now := time.Now()

			for id, member := range group.Members {
				if now.Sub(member.LastHeartbeat) > member.SessionTimeout {
					expiredMembers = append(expiredMembers, id)
					fmt.Printf("[Kafka] Member %s heartbeat expired in group %s\n", id, group.ID)
				}
			}

			// Remove expired members
			for _, id := range expiredMembers {
				delete(group.Members, id)
			}

			// Trigger rebalance if members expired
			if len(expiredMembers) > 0 {
				if len(group.Members) == 0 {
					group.State = StateEmpty
					group.Leader = ""
					group.Generation = 0
				} else {
					group.State = StatePreparingRebalance
					group.Generation++

					// Elect new leader if needed
					leaderExists := group.Leader != "" && group.Members[group.Leader] != nil

					if !leaderExists {
						for id := range group.Members {
							group.Leader = id
							break
						}
					}
				}
			}

			group.mutex.Unlock()
		}
	}
}

// getOrCreateGroup gets or creates a consumer group
func (gc *GroupCoordinator) getOrCreateGroup(groupID string) *ConsumerGroup {
	group, exists := gc.groups[groupID]
	if !exists {
		group = &ConsumerGroup{
			ID:               groupID,
			State:            StateEmpty,
			Members:          make(map[string]*GroupMember),
			Offsets:          make(map[TopicPartition]int64),
			Assignments:      make(map[string][]TopicPartition),
			ProtocolMetadata: make(map[string][]byte),
			CreatedAt:        time.Now(),
			Generation:       0,
		}
		gc.groups[groupID] = group
		fmt.Printf("[Kafka] Created new consumer group: %s\n", groupID)
	}
	return group
}

// Response types

type JoinGroupResponse struct {
	GenerationID int32
	ProtocolName string
	LeaderID     string
	MemberID     string
	Members      []GroupMemberInfo
}

type GroupMemberInfo struct {
	MemberID string
	Metadata []byte
}

type SyncGroupResponse struct {
	Assignment []byte
}

type GroupDescription struct {
	GroupID      string
	State        string
	ProtocolType string
	Protocol     string
	Members      []MemberDescription
	Generation   int32
}

type MemberDescription struct {
	MemberID   string
	ClientID   string
	ClientHost string
	Metadata   []byte
	Assignment []byte
}

type GroupOverview struct {
	GroupID      string
	ProtocolType string
	State        string
}

// RoundRobin partition assignment
func (r *RoundRobinRebalancePolicy) Assign(members []string, partitions map[string][]int32) map[string][]PartitionAssignment {
	assignments := make(map[string][]PartitionAssignment)

	// Initialize assignments
	for _, member := range members {
		assignments[member] = make([]PartitionAssignment, 0)
	}

	// Flatten all partitions
	type topicPartition struct {
		topic     string
		partition int32
	}
	allPartitions := make([]topicPartition, 0)

	for topic, parts := range partitions {
		for _, part := range parts {
			allPartitions = append(allPartitions, topicPartition{topic, part})
		}
	}

	// Distribute partitions in round-robin fashion
	for i, tp := range allPartitions {
		memberIdx := i % len(members)
		member := members[memberIdx]

		// Find or create topic assignment
		found := false
		for j := range assignments[member] {
			if assignments[member][j].Topic == tp.topic {
				assignments[member][j].Partitions = append(assignments[member][j].Partitions, tp.partition)
				found = true
				break
			}
		}

		if !found {
			assignments[member] = append(assignments[member], PartitionAssignment{
				Topic:      tp.topic,
				Partitions: []int32{tp.partition},
			})
		}
	}

	return assignments
}

// Range partition assignment
func (r *RangeRebalancePolicy) Assign(members []string, partitions map[string][]int32) map[string][]PartitionAssignment {
	assignments := make(map[string][]PartitionAssignment)

	// Initialize assignments
	for _, member := range members {
		assignments[member] = make([]PartitionAssignment, 0)
	}

	// Assign partitions topic by topic
	for topic, parts := range partitions {
		partCount := len(parts)
		memberCount := len(members)
		partitionsPerMember := partCount / memberCount
		remainder := partCount % memberCount

		partIdx := 0
		for i, member := range members {
			count := partitionsPerMember
			if i < remainder {
				count++
			}

			if count > 0 {
				assigned := parts[partIdx : partIdx+count]
				assignments[member] = append(assignments[member], PartitionAssignment{
					Topic:      topic,
					Partitions: assigned,
				})
				partIdx += count
			}
		}
	}

	return assignments
}

