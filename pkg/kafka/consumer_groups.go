package kafka

import (
	"fmt"
	"sync"
	"time"
)

// Advanced Kafka features for production compatibility

// ConsumerGroup represents a Kafka consumer group
type ConsumerGroup struct {
	ID               string
	Members          map[string]*GroupMember
	State            ConsumerGroupStateEnum
	Protocol         string
	Leader           string
	Generation       int32
	ProtocolMetadata map[string][]byte
	Assignments      map[string][]TopicPartition
	Offsets          map[TopicPartition]int64
	SessionTimeout   time.Duration
	RebalanceTimeout time.Duration
	CreatedAt        time.Time
	LastHeartbeat    time.Time
	mutex            sync.RWMutex
}

// GroupMember represents a member of a consumer group
type GroupMember struct {
	ID               string
	ClientID         string
	ClientHost       string
	SessionTimeout   time.Duration
	RebalanceTimeout time.Duration
	Subscription     []string
	Assignment       []TopicPartition
	Metadata         []byte
	JoinedAt         time.Time
	LastHeartbeat    time.Time
}

// ConsumerGroupStateEnum represents the state of a consumer group
type ConsumerGroupStateEnum int32

const (
	ConsumerGroupStateUnknown ConsumerGroupStateEnum = iota
	ConsumerGroupStatePreparingRebalance
	ConsumerGroupStateCompletingRebalance
	ConsumerGroupStateStable
	ConsumerGroupStateDead
	ConsumerGroupStateEmpty
)

func (s ConsumerGroupStateEnum) String() string {
	switch s {
	case ConsumerGroupStatePreparingRebalance:
		return "PreparingRebalance"
	case ConsumerGroupStateCompletingRebalance:
		return "CompletingRebalance"
	case ConsumerGroupStateStable:
		return "Stable"
	case ConsumerGroupStateDead:
		return "Dead"
	case ConsumerGroupStateEmpty:
		return "Empty"
	default:
		return "Unknown"
	}
}

// TopicPartition represents a topic and partition combination
type TopicPartition struct {
	Topic     string
	Partition int32
}

// OffsetCommitRequest represents an offset commit request
type OffsetCommitRequest struct {
	GroupID       string
	GenerationID  int32
	MemberID      string
	RetentionTime int64
	Topics        map[string][]PartitionOffsetCommit
}

// PartitionOffsetCommit represents partition offset to commit
type PartitionOffsetCommit struct {
	Partition int32
	Offset    int64
	Metadata  string
}

// OffsetFetchRequest represents an offset fetch request
type OffsetFetchRequest struct {
	GroupID string
	Topics  map[string][]int32
}

// Transaction represents a Kafka transaction
type Transaction struct {
	TransactionalID string
	ProducerID      int64
	ProducerEpoch   int16
	State           TransactionState
	Topics          map[string][]int32
	Groups          []string
	Timeout         time.Duration
	StartTime       time.Time
	LastUpdate      time.Time
}

// TransactionState represents the state of a transaction
type TransactionState int

const (
	TransactionStateEmpty TransactionState = iota
	TransactionStateOngoing
	TransactionStateCompleteCommit
	TransactionStateCompleteAbort
	TransactionStatePreparingRebalance
	TransactionStatePreparingCommit
	TransactionStateCommittingTxn
	TransactionStatePreparingAbort
	TransactionStateCompletingAbort
	TransactionStatePrepareCommit
	TransactionStatePrepareAbort
	TransactionStateDead
)

// PartitionAssignmentStrategy defines how partitions are assigned to consumers
type PartitionAssignmentStrategy interface {
	Name() string
	Assign(members map[string]*GroupMember, topics map[string][]int32) (map[string][]TopicPartition, error)
}

// RoundRobinAssignor implements round-robin partition assignment
type RoundRobinAssignor struct{}

func (r *RoundRobinAssignor) Name() string {
	return "RoundRobin"
}

func (r *RoundRobinAssignor) Assign(members map[string]*GroupMember, topics map[string][]int32) (map[string][]TopicPartition, error) {
	assignments := make(map[string][]TopicPartition)

	// Initialize assignments for all members
	for memberID := range members {
		assignments[memberID] = []TopicPartition{}
	}

	// Collect all topic-partitions
	var allPartitions []TopicPartition
	for topic, partitions := range topics {
		for _, partition := range partitions {
			allPartitions = append(allPartitions, TopicPartition{
				Topic:     topic,
				Partition: partition,
			})
		}
	}

	// Round-robin assignment
	memberIDs := make([]string, 0, len(members))
	for memberID := range members {
		memberIDs = append(memberIDs, memberID)
	}

	if len(memberIDs) == 0 {
		return assignments, nil
	}

	for i, partition := range allPartitions {
		memberID := memberIDs[i%len(memberIDs)]
		assignments[memberID] = append(assignments[memberID], partition)
	}

	return assignments, nil
}

// RangeAssignor implements range-based partition assignment
type RangeAssignor struct{}

func (r *RangeAssignor) Name() string {
	return "Range"
}

func (r *RangeAssignor) Assign(members map[string]*ConsumerGroupMember, topics map[string][]int32) (map[string][]TopicPartition, error) {
	assignments := make(map[string][]TopicPartition)

	// Initialize assignments for all members
	for memberID := range members {
		assignments[memberID] = []TopicPartition{}
	}

	memberIDs := make([]string, 0, len(members))
	for memberID := range members {
		memberIDs = append(memberIDs, memberID)
	}

	if len(memberIDs) == 0 {
		return assignments, nil
	}

	// Assign partitions for each topic separately
	for topic, partitions := range topics {
		partitionsPerMember := len(partitions) / len(memberIDs)
		remainder := len(partitions) % len(memberIDs)

		currentPartition := 0
		for i, memberID := range memberIDs {
			numPartitions := partitionsPerMember
			if i < remainder {
				numPartitions++
			}

			for j := 0; j < numPartitions && currentPartition < len(partitions); j++ {
				assignments[memberID] = append(assignments[memberID], TopicPartition{
					Topic:     topic,
					Partition: partitions[currentPartition],
				})
				currentPartition++
			}
		}
	}

	return assignments, nil
}

// ConsumerGroupCoordinator manages consumer groups
type ConsumerGroupCoordinator struct {
	groups             map[string]*ConsumerGroup
	assignmentStrategy PartitionAssignmentStrategy
	mutex              sync.RWMutex
	sessionTimeout     time.Duration
	rebalanceTimeout   time.Duration
	heartbeatInterval  time.Duration
}

// NewConsumerGroupCoordinator creates a new consumer group coordinator
func NewConsumerGroupCoordinator() *ConsumerGroupCoordinator {
	return &ConsumerGroupCoordinator{
		groups:             make(map[string]*ConsumerGroup),
		assignmentStrategy: &RoundRobinAssignor{}, // Default strategy
		sessionTimeout:     30 * time.Second,
		rebalanceTimeout:   60 * time.Second,
		heartbeatInterval:  3 * time.Second,
	}
}

// JoinGroup handles a consumer joining a group
func (c *ConsumerGroupCoordinator) JoinGroup(
	groupID, memberID, clientID, protocolType string,
	sessionTimeout, rebalanceTimeout time.Duration,
	protocols map[string][]byte,
) (*ConsumerGroup, string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	group, exists := c.groups[groupID]
	if !exists {
		group = &ConsumerGroup{
			ID:               groupID,
			Members:          make(map[string]*GroupMember),
			State:            ConsumerGroupStateEmpty,
			Protocol:         protocolType,
			Assignments:      make(map[string][]TopicPartition),
			Offsets:          make(map[TopicPartition]int64),
			SessionTimeout:   sessionTimeout,
			RebalanceTimeout: rebalanceTimeout,
			CreatedAt:        time.Now(),
		}
		c.groups[groupID] = group
	}

	// Generate member ID if not provided
	if memberID == "" {
		memberID = fmt.Sprintf("%s-%d", clientID, time.Now().UnixNano())
	}

	member := &GroupMember{
		ID:               memberID,
		ClientID:         clientID,
		SessionTimeout:   sessionTimeout,
		RebalanceTimeout: rebalanceTimeout,
		JoinedAt:         time.Now(),
		LastHeartbeat:    time.Now(),
	}

	group.Members[memberID] = member
	group.State = ConsumerGroupStatePreparingRebalance
	group.Generation++

	return group, memberID, nil
}

// SyncGroup handles group synchronization
func (c *ConsumerGroupCoordinator) SyncGroup(groupID, memberID string, generation int32, assignments map[string][]TopicPartition) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	group, exists := c.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	if group.Generation != generation {
		return fmt.Errorf("invalid generation: expected %d, got %d", group.Generation, generation)
	}

	// Update assignments
	group.Assignments = assignments
	group.State = ConsumerGroupStateStable

	return nil
}

// Heartbeat handles consumer heartbeats
func (c *ConsumerGroupCoordinator) Heartbeat(groupID, memberID string, generation int32) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	group, exists := c.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	member, exists := group.Members[memberID]
	if !exists {
		return fmt.Errorf("member not found: %s", memberID)
	}

	if group.Generation != generation {
		return fmt.Errorf("invalid generation: expected %d, got %d", group.Generation, generation)
	}

	member.LastHeartbeat = time.Now()
	group.LastHeartbeat = time.Now()

	return nil
}

// LeaveGroup handles a consumer leaving a group
func (c *ConsumerGroupCoordinator) LeaveGroup(groupID, memberID string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	group, exists := c.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	delete(group.Members, memberID)

	if len(group.Members) == 0 {
		group.State = ConsumerGroupStateEmpty
	} else {
		group.State = ConsumerGroupStatePreparingRebalance
		group.Generation++
	}

	return nil
}

// CommitOffset commits offsets for a consumer group
func (c *ConsumerGroupCoordinator) CommitOffset(groupID string, commits map[TopicPartition]int64) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	group, exists := c.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	for tp, offset := range commits {
		group.Offsets[tp] = offset
	}

	return nil
}

// FetchOffsets fetches committed offsets for a consumer group
func (c *ConsumerGroupCoordinator) FetchOffsets(groupID string, partitions []TopicPartition) (map[TopicPartition]int64, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	group, exists := c.groups[groupID]
	if !exists {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}

	offsets := make(map[TopicPartition]int64)
	for _, tp := range partitions {
		if offset, exists := group.Offsets[tp]; exists {
			offsets[tp] = offset
		} else {
			offsets[tp] = -1 // Unknown offset
		}
	}

	return offsets, nil
}

// ListGroups returns all consumer groups
func (c *ConsumerGroupCoordinator) ListGroups() map[string]*ConsumerGroup {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	groups := make(map[string]*ConsumerGroup)
	for id, group := range c.groups {
		groups[id] = group
	}

	return groups
}

// DescribeGroups returns detailed information about consumer groups
func (c *ConsumerGroupCoordinator) DescribeGroups(groupIDs []string) map[string]*ConsumerGroup {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	groups := make(map[string]*ConsumerGroup)
	for _, groupID := range groupIDs {
		if group, exists := c.groups[groupID]; exists {
			groups[groupID] = group
		}
	}

	return groups
}

// StartRebalance triggers a rebalance for a consumer group
func (c *ConsumerGroupCoordinator) StartRebalance(groupID string, topics map[string][]int32) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	group, exists := c.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	if group.State != ConsumerGroupStateStable {
		return fmt.Errorf("group not in stable state: %s", group.State)
	}

	group.State = ConsumerGroupStatePreparingRebalance
	group.Generation++

	// Calculate new assignments
	assignments, err := c.assignmentStrategy.Assign(group.Members, topics)
	if err != nil {
		return fmt.Errorf("failed to assign partitions: %w", err)
	}

	group.Assignments = assignments
	group.State = ConsumerGroupStateStable

	return nil
}

// CleanupExpiredGroups removes expired consumer groups
func (c *ConsumerGroupCoordinator) CleanupExpiredGroups() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for groupID, group := range c.groups {
		// Check if group has expired members
		hasExpiredMembers := false
		for memberID, member := range group.Members {
			if now.Sub(member.LastHeartbeat) > group.SessionTimeout {
				delete(group.Members, memberID)
				hasExpiredMembers = true
			}
		}

		// Update group state if members expired
		if hasExpiredMembers {
			if len(group.Members) == 0 {
				group.State = ConsumerGroupStateEmpty
			} else {
				group.State = ConsumerGroupStatePreparingRebalance
				group.Generation++
			}
		}

		// Remove empty groups that have been inactive
		if len(group.Members) == 0 && now.Sub(group.LastHeartbeat) > 5*time.Minute {
			delete(c.groups, groupID)
		}
	}
}
