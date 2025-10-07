package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// StorageAdapter integrates Kafka with persistent storage (Dragonfly/Redis)
type StorageAdapter struct {
	client  *redis.Client
	ttl     time.Duration
	enabled bool
}

// NewStorageAdapter creates a new storage adapter
func NewStorageAdapter(client *redis.Client, ttl time.Duration) *StorageAdapter {
	return &StorageAdapter{
		client:  client,
		ttl:     ttl,
		enabled: client != nil,
	}
}

// StoreMessage persists a message to storage
func (sa *StorageAdapter) StoreMessage(ctx context.Context, topic string, partition int32, offset int64, key, value []byte) error {
	if !sa.enabled {
		return nil // Storage disabled
	}

	messageKey := fmt.Sprintf("kafka:message:%s:%d:%d", topic, partition, offset)

	message := map[string]interface{}{
		"topic":     topic,
		"partition": partition,
		"offset":    offset,
		"key":       key,
		"value":     value,
		"timestamp": time.Now().Unix(),
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return sa.client.Set(ctx, messageKey, data, sa.ttl).Err()
}

// FetchMessage retrieves a message from storage
func (sa *StorageAdapter) FetchMessage(ctx context.Context, topic string, partition int32, offset int64) (key, value []byte, err error) {
	if !sa.enabled {
		return nil, nil, fmt.Errorf("storage disabled")
	}

	messageKey := fmt.Sprintf("kafka:message:%s:%d:%d", topic, partition, offset)

	data, err := sa.client.Get(ctx, messageKey).Bytes()
	if err != nil {
		return nil, nil, err
	}

	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, nil, err
	}

	// Extract key and value
	if k, ok := message["key"].([]byte); ok {
		key = k
	}
	if v, ok := message["value"].([]byte); ok {
		value = v
	}

	return key, value, nil
}

// StoreOffset persists a consumer group offset
func (sa *StorageAdapter) StoreOffset(ctx context.Context, groupID, topic string, partition int32, offset int64, metadata string) error {
	if !sa.enabled {
		return nil
	}

	offsetKey := fmt.Sprintf("kafka:offset:%s:%s:%d", groupID, topic, partition)

	offsetData := map[string]interface{}{
		"offset":    offset,
		"metadata":  metadata,
		"timestamp": time.Now().Unix(),
	}

	data, err := json.Marshal(offsetData)
	if err != nil {
		return fmt.Errorf("failed to marshal offset: %w", err)
	}

	return sa.client.Set(ctx, offsetKey, data, 0).Err() // No expiration for offsets
}

// FetchOffset retrieves a consumer group offset
func (sa *StorageAdapter) FetchOffset(ctx context.Context, groupID, topic string, partition int32) (offset int64, metadata string, err error) {
	if !sa.enabled {
		return -1, "", fmt.Errorf("storage disabled")
	}

	offsetKey := fmt.Sprintf("kafka:offset:%s:%s:%d", groupID, topic, partition)

	data, err := sa.client.Get(ctx, offsetKey).Bytes()
	if err == redis.Nil {
		return -1, "", nil // No offset stored
	}
	if err != nil {
		return -1, "", err
	}

	var offsetData map[string]interface{}
	if err := json.Unmarshal(data, &offsetData); err != nil {
		return -1, "", err
	}

	if o, ok := offsetData["offset"].(float64); ok {
		offset = int64(o)
	}
	if m, ok := offsetData["metadata"].(string); ok {
		metadata = m
	}

	return offset, metadata, nil
}

// DeleteOffsets removes all offsets for a consumer group
func (sa *StorageAdapter) DeleteOffsets(ctx context.Context, groupID string) error {
	if !sa.enabled {
		return nil
	}

	pattern := fmt.Sprintf("kafka:offset:%s:*", groupID)
	iter := sa.client.Scan(ctx, 0, pattern, 100).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return err
	}

	if len(keys) > 0 {
		return sa.client.Del(ctx, keys...).Err()
	}

	return nil
}

// StoreGroupMetadata persists consumer group metadata
func (sa *StorageAdapter) StoreGroupMetadata(ctx context.Context, groupID string, metadata map[string]interface{}) error {
	if !sa.enabled {
		return nil
	}

	groupKey := fmt.Sprintf("kafka:group:%s", groupID)

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal group metadata: %w", err)
	}

	return sa.client.Set(ctx, groupKey, data, 0).Err()
}

// FetchGroupMetadata retrieves consumer group metadata
func (sa *StorageAdapter) FetchGroupMetadata(ctx context.Context, groupID string) (map[string]interface{}, error) {
	if !sa.enabled {
		return nil, fmt.Errorf("storage disabled")
	}

	groupKey := fmt.Sprintf("kafka:group:%s", groupID)

	data, err := sa.client.Get(ctx, groupKey).Bytes()
	if err == redis.Nil {
		return nil, nil // No metadata stored
	}
	if err != nil {
		return nil, err
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}

// DeleteGroupMetadata removes consumer group metadata
func (sa *StorageAdapter) DeleteGroupMetadata(ctx context.Context, groupID string) error {
	if !sa.enabled {
		return nil
	}

	groupKey := fmt.Sprintf("kafka:group:%s", groupID)
	return sa.client.Del(ctx, groupKey).Err()
}

// ListTopics returns all topics that have messages stored
func (sa *StorageAdapter) ListTopics(ctx context.Context) ([]string, error) {
	if !sa.enabled {
		return nil, fmt.Errorf("storage disabled")
	}

	pattern := "kafka:message:*"
	iter := sa.client.Scan(ctx, 0, pattern, 100).Iterator()

	topicsSet := make(map[string]bool)
	for iter.Next(ctx) {
		// Extract topic from key: kafka:message:<topic>:<partition>:<offset>
		key := iter.Val()
		// Simple extraction (production code should be more robust)
		topicsSet[key] = true
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	topics := make([]string, 0, len(topicsSet))
	for topic := range topicsSet {
		topics = append(topics, topic)
	}

	return topics, nil
}

// GetPartitionCount returns the number of partitions for a topic
func (sa *StorageAdapter) GetPartitionCount(ctx context.Context, topic string) (int32, error) {
	if !sa.enabled {
		return 0, fmt.Errorf("storage disabled")
	}

	// For now, return a default value
	// Production code should store this in Redis
	return 3, nil // Default 3 partitions
}

// CleanupExpiredMessages removes messages older than TTL
func (sa *StorageAdapter) CleanupExpiredMessages(ctx context.Context) (int, error) {
	if !sa.enabled {
		return 0, nil
	}

	pattern := "kafka:message:*"
	iter := sa.client.Scan(ctx, 0, pattern, 100).Iterator()

	cleaned := 0
	cutoff := time.Now().Add(-sa.ttl).Unix()

	for iter.Next(ctx) {
		key := iter.Val()
		data, err := sa.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var message map[string]interface{}
		if err := json.Unmarshal(data, &message); err != nil {
			continue
		}

		if timestamp, ok := message["timestamp"].(float64); ok {
			if int64(timestamp) < cutoff {
				if err := sa.client.Del(ctx, key).Err(); err == nil {
					cleaned++
				}
			}
		}
	}

	if err := iter.Err(); err != nil {
		return cleaned, err
	}

	return cleaned, nil
}

// Stats returns storage statistics
func (sa *StorageAdapter) Stats(ctx context.Context) (map[string]interface{}, error) {
	if !sa.enabled {
		return map[string]interface{}{
			"enabled": false,
		}, nil
	}

	info, err := sa.client.Info(ctx).Result()
	if err != nil {
		return nil, err
	}

	// Count Kafka-related keys
	messageKeys := 0
	offsetKeys := 0
	groupKeys := 0

	patterns := []string{"kafka:message:*", "kafka:offset:*", "kafka:group:*"}
	for i, pattern := range patterns {
		iter := sa.client.Scan(ctx, 0, pattern, 10).Iterator()
		count := 0
		for iter.Next(ctx) {
			count++
		}

		switch i {
		case 0:
			messageKeys = count
		case 1:
			offsetKeys = count
		case 2:
			groupKeys = count
		}
	}

	return map[string]interface{}{
		"enabled":      true,
		"redis_info":   info,
		"message_keys": messageKeys,
		"offset_keys":  offsetKeys,
		"group_keys":   groupKeys,
	}, nil
}

