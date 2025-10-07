package kafka

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

var (
	ErrGroupNotFound     = errors.New("group not found")
	ErrUnknownMemberID   = errors.New("unknown member ID")
	ErrIllegalGeneration = errors.New("illegal generation")
)

// ShardedGroupCoordinator manages consumer groups with lock sharding for reduced contention
type ShardedGroupCoordinator struct {
	shards    []*groupShard
	numShards int
}

type groupShard struct {
	mu     sync.RWMutex
	groups map[string]*ConsumerGroup
}

// NewShardedGroupCoordinator creates a new sharded group coordinator
func NewShardedGroupCoordinator() *ShardedGroupCoordinator {
	return NewShardedGroupCoordinatorWithShards(defaultNumShards)
}

// NewShardedGroupCoordinatorWithShards creates a new sharded group coordinator with custom shard count
func NewShardedGroupCoordinatorWithShards(numShards int) *ShardedGroupCoordinator {
	shards := make([]*groupShard, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = &groupShard{
			groups: make(map[string]*ConsumerGroup),
		}
	}

	return &ShardedGroupCoordinator{
		shards:    shards,
		numShards: numShards,
	}
}

// getShardHash calculates shard index based on group ID
func (gc *ShardedGroupCoordinator) getShardHash(groupID string) int {
	h := fnv.New32a()
	h.Write([]byte(groupID))
	return int(h.Sum32()) % gc.numShards
}

// getShard returns the shard for a given group ID
func (gc *ShardedGroupCoordinator) getShard(groupID string) *groupShard {
	return gc.shards[gc.getShardHash(groupID)]
}

// JoinGroup handles a consumer joining a group
func (gc *ShardedGroupCoordinator) JoinGroup(
	groupID, memberID, clientID, clientHost, protocolType string,
	sessionTimeout, rebalanceTimeout time.Duration,
	protocols []string,
	metadata []byte,
) (*JoinGroupResponse, error) {
	shard := gc.getShard(groupID)
	
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Get or create group
	group, exists := shard.groups[groupID]
	if !exists {
		group = &ConsumerGroup{
			ID:               groupID,
			State:            StateEmpty,
			Members:          make(map[string]*GroupMember),
			Generation:       0,
			Protocol:         protocolType,
			SessionTimeout:   sessionTimeout,
			RebalanceTimeout: rebalanceTimeout,
			CreatedAt:        time.Now(),
		}
		shard.groups[groupID] = group
	}

	// Generate member ID if not provided
	if memberID == "" {
		memberID = fmt.Sprintf("%s-%s-%d", clientID, clientHost, time.Now().UnixNano())
	}

	// Add member to group
	member := &GroupMember{
		ID:               memberID,
		ClientID:         clientID,
		ClientHost:       clientHost,
		SessionTimeout:   sessionTimeout,
		RebalanceTimeout: rebalanceTimeout,
		LastHeartbeat:    time.Now(),
		JoinedAt:         time.Now(),
		Subscription:     protocols,
		Metadata:         metadata,
	}
	group.Members[memberID] = member

	// Update group state
	if group.State == StateEmpty {
		group.State = StatePreparingRebalance
	}

	// Select leader (first member)
	leaderID := memberID
	if group.Leader != "" {
		leaderID = group.Leader
	} else {
		group.Leader = memberID
		group.Generation++
	}

	response := &JoinGroupResponse{
		GenerationID: group.Generation,
		ProtocolName: group.Protocol,
		LeaderID:     leaderID,
		MemberID:     memberID,
		Members:      make([]GroupMemberInfo, 0, len(group.Members)),
	}

	// If this is the leader, include all members
	if memberID == leaderID {
		for _, m := range group.Members {
			response.Members = append(response.Members, GroupMemberInfo{
				MemberID: m.ID,
				Metadata: m.Metadata,
			})
		}
	}

	return response, nil
}

// SyncGroup handles synchronization of member assignments
func (gc *ShardedGroupCoordinator) SyncGroup(
	groupID, memberID string,
	generationID int32,
	assignments map[string][]byte,
) (*SyncGroupResponse, error) {
	shard := gc.getShard(groupID)
	
	shard.mu.Lock()
	defer shard.mu.Unlock()

	group, exists := shard.groups[groupID]
	if !exists {
		return nil, ErrGroupNotFound
	}

	// Verify generation
	if group.Generation != generationID {
		return nil, ErrIllegalGeneration
	}

	// Update member assignments (convert []byte to []TopicPartition if needed)
	for memID, assignment := range assignments {
		if member, exists := group.Members[memID]; exists {
			// Store assignment as byte array - will be parsed by consumer
			_ = assignment // For now, skip assignment parsing
			member.Assignment = []TopicPartition{} // Empty for now
		}
	}

	// Update group state
	group.State = StateStable

	// Get this member's assignment
	_, exists = group.Members[memberID]
	if !exists {
		return nil, ErrUnknownMemberID
	}

	// Return assignment as bytes - encode TopicPartition array
	assignmentBytes := make([]byte, 0) // TODO: Proper encoding
	
	return &SyncGroupResponse{
		Assignment: assignmentBytes,
	}, nil
}

// Heartbeat handles member heartbeat
func (gc *ShardedGroupCoordinator) Heartbeat(
	groupID, memberID string,
	generationID int32,
) error {
	shard := gc.getShard(groupID)
	
	shard.mu.Lock()
	defer shard.mu.Unlock()

	group, exists := shard.groups[groupID]
	if !exists {
		return ErrGroupNotFound
	}

	member, exists := group.Members[memberID]
	if !exists {
		return ErrUnknownMemberID
	}

	// Verify generation
	if group.Generation != generationID {
		return ErrIllegalGeneration
	}

	// Update last heartbeat
	member.LastHeartbeat = time.Now()

	return nil
}

// LeaveGroup handles a member leaving the group
func (gc *ShardedGroupCoordinator) LeaveGroup(groupID, memberID string) error {
	shard := gc.getShard(groupID)
	
	shard.mu.Lock()
	defer shard.mu.Unlock()

	group, exists := shard.groups[groupID]
	if !exists {
		return ErrGroupNotFound
	}

	// Remove member
	delete(group.Members, memberID)

	// If no members left, delete group
	if len(group.Members) == 0 {
		delete(shard.groups, groupID)
		return nil
	}

	// If leader left, trigger rebalance
	if group.Leader == memberID {
		group.State = StatePreparingRebalance
		// Select new leader (first member)
		for memID := range group.Members {
			group.Leader = memID
			break
		}
	}

	return nil
}

// DescribeGroups returns information about consumer groups
func (gc *ShardedGroupCoordinator) DescribeGroups(groupIDs []string) ([]*GroupDescription, error) {
	descriptions := make([]*GroupDescription, 0, len(groupIDs))

	for _, groupID := range groupIDs {
		shard := gc.getShard(groupID)
		
		shard.mu.RLock()
		group, exists := shard.groups[groupID]
		if !exists {
			shard.mu.RUnlock()
			// Return error info for non-existent group
			descriptions = append(descriptions, &GroupDescription{
				GroupID: groupID,
				State:   "Dead",
				Members: []MemberDescription{},
			})
			continue
		}

		// Build member descriptions
		members := make([]MemberDescription, 0, len(group.Members))
		for _, member := range group.Members {
			members = append(members, MemberDescription{
				MemberID:   member.ID,
				ClientID:   member.ClientID,
				ClientHost: member.ClientHost,
				Metadata:   member.Metadata,
			})
		}

		descriptions = append(descriptions, &GroupDescription{
			GroupID:    groupID,
			State:      string(group.State),
			Protocol:   group.Protocol,
			Members:    members,
			Generation: group.Generation,
		})
		
		shard.mu.RUnlock()
	}

	return descriptions, nil
}

// ListGroups returns all consumer groups
func (gc *ShardedGroupCoordinator) ListGroups() ([]GroupOverview, error) {
	groupSet := make(map[string]*ConsumerGroup)

	// Iterate all shards
	for _, shard := range gc.shards {
		shard.mu.RLock()
		for groupID, group := range shard.groups {
			groupSet[groupID] = group
		}
		shard.mu.RUnlock()
	}

	groups := make([]GroupOverview, 0, len(groupSet))
	for _, group := range groupSet {
		groups = append(groups, GroupOverview{
			GroupID: group.ID,
			State:   string(group.State),
		})
	}

	return groups, nil
}

// GetStats returns statistics about the group coordinator
func (gc *ShardedGroupCoordinator) GetStats() GroupCoordinatorStats {
	stats := GroupCoordinatorStats{
		ShardStats: make([]GroupShardStats, gc.numShards),
	}

	for i, shard := range gc.shards {
		shard.mu.RLock()
		
		shardStats := GroupShardStats{
			ShardID:    i,
			GroupCount: len(shard.groups),
		}

		for _, group := range shard.groups {
			shardStats.MemberCount += len(group.Members)
		}

		stats.ShardStats[i] = shardStats
		stats.TotalGroups += shardStats.GroupCount
		stats.TotalMembers += shardStats.MemberCount
		
		shard.mu.RUnlock()
	}

	return stats
}

// GroupCoordinatorStats provides statistics about the group coordinator
type GroupCoordinatorStats struct {
	TotalGroups  int
	TotalMembers int
	ShardStats   []GroupShardStats
}

// GroupShardStats provides statistics for a single shard
type GroupShardStats struct {
	ShardID     int
	GroupCount  int
	MemberCount int
}

