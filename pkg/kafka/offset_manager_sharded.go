package kafka

import (
	"errors"
	"hash/fnv"
	"sync"
	"time"
)

var (
	ErrOffsetNotFound = errors.New("offset not found")
)

// ShardedOffsetManager implements offset management with lock sharding for reduced contention
// Each shard has its own lock, allowing concurrent operations on different groups
type ShardedOffsetManager struct {
	shards    []*offsetShard
	numShards int
}

type offsetShard struct {
	mu      sync.RWMutex
	offsets map[string]map[string]map[int32]*OffsetMetadata // group -> topic -> partition -> offset
}

const defaultNumShards = 64 // Optimal for most workloads

// NewShardedOffsetManager creates a new sharded offset manager
func NewShardedOffsetManager() *ShardedOffsetManager {
	return NewShardedOffsetManagerWithShards(defaultNumShards)
}

// NewShardedOffsetManagerWithShards creates a new sharded offset manager with custom shard count
func NewShardedOffsetManagerWithShards(numShards int) *ShardedOffsetManager {
	shards := make([]*offsetShard, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = &offsetShard{
			offsets: make(map[string]map[string]map[int32]*OffsetMetadata),
		}
	}

	return &ShardedOffsetManager{
		shards:    shards,
		numShards: numShards,
	}
}

// getShardHash calculates shard index based on group ID
func (m *ShardedOffsetManager) getShardHash(groupID string) int {
	h := fnv.New32a()
	h.Write([]byte(groupID))
	return int(h.Sum32()) % m.numShards
}

// getShard returns the shard for a given group ID
func (m *ShardedOffsetManager) getShard(groupID string) *offsetShard {
	return m.shards[m.getShardHash(groupID)]
}

// CommitOffset commits an offset for a consumer group, topic, and partition
func (m *ShardedOffsetManager) CommitOffset(groupID, topic string, partition int32, offset int64) error {
	return m.CommitOffsetWithMetadata(groupID, topic, partition, &OffsetMetadata{
		Offset:    offset,
		Timestamp: time.Now(),
	})
}

// CommitOffsetWithMetadata commits an offset with metadata
func (m *ShardedOffsetManager) CommitOffsetWithMetadata(groupID, topic string, partition int32, offsetMetadata *OffsetMetadata) error {
	shard := m.getShard(groupID)
	
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if shard.offsets[groupID] == nil {
		shard.offsets[groupID] = make(map[string]map[int32]*OffsetMetadata)
	}
	if shard.offsets[groupID][topic] == nil {
		shard.offsets[groupID][topic] = make(map[int32]*OffsetMetadata)
	}

	shard.offsets[groupID][topic][partition] = offsetMetadata
	return nil
}

// FetchOffset retrieves the committed offset for a consumer group, topic, and partition
func (m *ShardedOffsetManager) FetchOffset(groupID, topic string, partition int32) (int64, error) {
	offsetMetadata, err := m.FetchOffsetWithMetadata(groupID, topic, partition)
	if err != nil {
		return -1, err
	}
	return offsetMetadata.Offset, nil
}

// FetchOffsetWithMetadata retrieves the committed offset with metadata
func (m *ShardedOffsetManager) FetchOffsetWithMetadata(groupID, topic string, partition int32) (*OffsetMetadata, error) {
	shard := m.getShard(groupID)
	
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if topicOffsets, exists := shard.offsets[groupID]; exists {
		if partitionOffsets, exists := topicOffsets[topic]; exists {
			if offsetMetadata, exists := partitionOffsets[partition]; exists {
				return offsetMetadata, nil
			}
		}
	}

	return nil, ErrOffsetNotFound
}

// FetchAllOffsets retrieves all committed offsets for a consumer group
func (m *ShardedOffsetManager) FetchAllOffsets(groupID string) (map[string]map[int32]*OffsetMetadata, error) {
	shard := m.getShard(groupID)
	
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if topicOffsets, exists := shard.offsets[groupID]; exists {
		// Deep copy to prevent external modifications
		result := make(map[string]map[int32]*OffsetMetadata)
		for topic, partitions := range topicOffsets {
			result[topic] = make(map[int32]*OffsetMetadata)
			for partition, offsetMetadata := range partitions {
				result[topic][partition] = &OffsetMetadata{
					Offset:    offsetMetadata.Offset,
					Metadata:  offsetMetadata.Metadata,
					Timestamp: offsetMetadata.Timestamp,
				}
			}
		}
		return result, nil
	}

	return make(map[string]map[int32]*OffsetMetadata), nil
}

// DeleteGroup removes all offsets for a consumer group
func (m *ShardedOffsetManager) DeleteGroup(groupID string) error {
	shard := m.getShard(groupID)
	
	shard.mu.Lock()
	defer shard.mu.Unlock()

	delete(shard.offsets, groupID)
	return nil
}

// ListGroups returns a list of all consumer groups with committed offsets
func (m *ShardedOffsetManager) ListGroups() ([]string, error) {
	// Need to iterate all shards
	groupSet := make(map[string]struct{})

	for _, shard := range m.shards {
		shard.mu.RLock()
		for groupID := range shard.offsets {
			groupSet[groupID] = struct{}{}
		}
		shard.mu.RUnlock()
	}

	groups := make([]string, 0, len(groupSet))
	for groupID := range groupSet {
		groups = append(groups, groupID)
	}

	return groups, nil
}

// ListGroupTopics returns all topics for which a consumer group has committed offsets
func (m *ShardedOffsetManager) ListGroupTopics(groupID string) ([]string, error) {
	shard := m.getShard(groupID)
	
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if topicOffsets, exists := shard.offsets[groupID]; exists {
		topics := make([]string, 0, len(topicOffsets))
		for topic := range topicOffsets {
			topics = append(topics, topic)
		}
		return topics, nil
	}

	return []string{}, nil
}

// GetStats returns statistics about the offset manager
func (m *ShardedOffsetManager) GetStats() OffsetManagerStats {
	stats := OffsetManagerStats{
		ShardStats: make([]ShardStats, m.numShards),
	}

	for i, shard := range m.shards {
		shard.mu.RLock()
		
		shardStats := ShardStats{
			ShardID:    i,
			GroupCount: len(shard.offsets),
		}

		for _, topicOffsets := range shard.offsets {
			for _, partitionOffsets := range topicOffsets {
				shardStats.OffsetCount += len(partitionOffsets)
			}
		}

		stats.ShardStats[i] = shardStats
		stats.TotalGroups += shardStats.GroupCount
		stats.TotalOffsets += shardStats.OffsetCount
		
		shard.mu.RUnlock()
	}

	return stats
}

// OffsetManagerStats provides statistics about the offset manager
type OffsetManagerStats struct {
	TotalGroups  int
	TotalOffsets int
	ShardStats   []ShardStats
}

// ShardStats provides statistics for a single shard
type ShardStats struct {
	ShardID     int
	GroupCount  int
	OffsetCount int
}

