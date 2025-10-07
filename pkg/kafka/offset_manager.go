package kafka

import (
	"fmt"
	"sync"
	"time"
)

// OffsetManager manages consumer group offsets
type OffsetManager struct {
	offsets map[string]map[string]map[int32]int64 // groupID -> topic -> partition -> offset
	mutex   sync.RWMutex
}

// NewOffsetManager creates a new offset manager
func NewOffsetManager() *OffsetManager {
	return &OffsetManager{
		offsets: make(map[string]map[string]map[int32]int64),
	}
}

// CommitOffset commits an offset for a consumer group
func (om *OffsetManager) CommitOffset(groupID, topic string, partition int32, offset int64) error {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	if om.offsets[groupID] == nil {
		om.offsets[groupID] = make(map[string]map[int32]int64)
	}

	if om.offsets[groupID][topic] == nil {
		om.offsets[groupID][topic] = make(map[int32]int64)
	}

	om.offsets[groupID][topic][partition] = offset
	return nil
}

// FetchOffset fetches the committed offset for a consumer group
func (om *OffsetManager) FetchOffset(groupID, topic string, partition int32) (int64, error) {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	if om.offsets[groupID] == nil {
		return -1, fmt.Errorf("group not found: %s", groupID)
	}

	if om.offsets[groupID][topic] == nil {
		return -1, fmt.Errorf("topic not found: %s", topic)
	}

	offset, exists := om.offsets[groupID][topic][partition]
	if !exists {
		return -1, fmt.Errorf("partition offset not found")
	}

	return offset, nil
}

// FetchAllOffsets fetches all offsets for a consumer group
func (om *OffsetManager) FetchAllOffsets(groupID string) (map[string]map[int32]int64, error) {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	if om.offsets[groupID] == nil {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}

	// Deep copy to avoid race conditions
	result := make(map[string]map[int32]int64)
	for topic, partitions := range om.offsets[groupID] {
		result[topic] = make(map[int32]int64)
		for partition, offset := range partitions {
			result[topic][partition] = offset
		}
	}

	return result, nil
}

// DeleteGroup deletes all offsets for a consumer group
func (om *OffsetManager) DeleteGroup(groupID string) error {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	if om.offsets[groupID] == nil {
		return fmt.Errorf("group not found: %s", groupID)
	}

	delete(om.offsets, groupID)
	return nil
}

// ListGroups returns all consumer group IDs
func (om *OffsetManager) ListGroups() []string {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	groups := make([]string, 0, len(om.offsets))
	for groupID := range om.offsets {
		groups = append(groups, groupID)
	}

	return groups
}

// GetGroupTopics returns all topics for a consumer group
func (om *OffsetManager) GetGroupTopics(groupID string) ([]string, error) {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	if om.offsets[groupID] == nil {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}

	topics := make([]string, 0, len(om.offsets[groupID]))
	for topic := range om.offsets[groupID] {
		topics = append(topics, topic)
	}

	return topics, nil
}

// OffsetMetadata contains metadata about an offset
type OffsetMetadata struct {
	Offset    int64
	Metadata  string
	Timestamp time.Time
}

// OffsetManagerWithMetadata extends OffsetManager with metadata support
type OffsetManagerWithMetadata struct {
	*OffsetManager
	metadata map[string]map[string]map[int32]*OffsetMetadata // groupID -> topic -> partition -> metadata
	metaMux  sync.RWMutex
}

// NewOffsetManagerWithMetadata creates a new offset manager with metadata support
func NewOffsetManagerWithMetadata() *OffsetManagerWithMetadata {
	return &OffsetManagerWithMetadata{
		OffsetManager: NewOffsetManager(),
		metadata:      make(map[string]map[string]map[int32]*OffsetMetadata),
	}
}

// CommitOffsetWithMetadata commits an offset with metadata
func (omm *OffsetManagerWithMetadata) CommitOffsetWithMetadata(groupID, topic string, partition int32, offset int64, metadata string) error {
	// Commit the offset
	if err := omm.CommitOffset(groupID, topic, partition, offset); err != nil {
		return err
	}

	// Store metadata
	omm.metaMux.Lock()
	defer omm.metaMux.Unlock()

	if omm.metadata[groupID] == nil {
		omm.metadata[groupID] = make(map[string]map[int32]*OffsetMetadata)
	}

	if omm.metadata[groupID][topic] == nil {
		omm.metadata[groupID][topic] = make(map[int32]*OffsetMetadata)
	}

	omm.metadata[groupID][topic][partition] = &OffsetMetadata{
		Offset:    offset,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	return nil
}

// FetchOffsetMetadata fetches offset metadata
func (omm *OffsetManagerWithMetadata) FetchOffsetMetadata(groupID, topic string, partition int32) (*OffsetMetadata, error) {
	omm.metaMux.RLock()
	defer omm.metaMux.RUnlock()

	if omm.metadata[groupID] == nil {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}

	if omm.metadata[groupID][topic] == nil {
		return nil, fmt.Errorf("topic not found: %s", topic)
	}

	meta, exists := omm.metadata[groupID][topic][partition]
	if !exists {
		return nil, fmt.Errorf("partition metadata not found")
	}

	return meta, nil
}

// CleanupExpiredOffsets removes offsets older than retention period
func (omm *OffsetManagerWithMetadata) CleanupExpiredOffsets(retention time.Duration) int {
	omm.metaMux.Lock()
	defer omm.metaMux.Unlock()

	cleaned := 0
	cutoff := time.Now().Add(-retention)

	for groupID, topics := range omm.metadata {
		for topic, partitions := range topics {
			for partition, meta := range partitions {
				if meta.Timestamp.Before(cutoff) {
					delete(partitions, partition)
					cleaned++
				}
			}

			// Clean up empty maps
			if len(partitions) == 0 {
				delete(topics, topic)
			}
		}

		if len(topics) == 0 {
			delete(omm.metadata, groupID)
			// Also delete from offset manager
			omm.OffsetManager.DeleteGroup(groupID)
		}
	}

	return cleaned
}

// Stats returns statistics about offset management
func (omm *OffsetManagerWithMetadata) Stats() map[string]interface{} {
	omm.mutex.RLock()
	omm.metaMux.RLock()
	defer omm.mutex.RUnlock()
	defer omm.metaMux.RUnlock()

	totalGroups := len(omm.offsets)
	totalOffsets := 0

	for _, topics := range omm.offsets {
		for _, partitions := range topics {
			totalOffsets += len(partitions)
		}
	}

	return map[string]interface{}{
		"total_groups":  totalGroups,
		"total_offsets": totalOffsets,
	}
}

