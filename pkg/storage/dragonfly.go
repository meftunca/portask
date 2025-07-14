package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/meftunca/portask/pkg/types"
	"github.com/redis/go-redis/v9"
)

// DragonflyStorage implements MessageStore using Dragonfly/Redis
type DragonflyStorage struct {
	client *redis.Client
	config *DragonflyStorageConfig
}

// DragonflyStorageConfig holds Dragonfly-specific configuration
type DragonflyStorageConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewDragonflyStorage creates a new Dragonfly storage adapter
func NewDragonflyStorage(config *DragonflyStorageConfig) (*DragonflyStorage, error) {
	opts := &redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Dragonfly: %w", err)
	}

	return &DragonflyStorage{
		client: client,
		config: config,
	}, nil
}

// Store implements MessageStore.Store using Redis Streams
func (d *DragonflyStorage) Store(ctx context.Context, message *types.PortaskMessage) error {
	// Convert message to JSON for storage
	messageData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Store in Redis Stream: STREAM_<topic>
	streamKey := fmt.Sprintf("STREAM_%s", message.Topic)

	values := map[string]interface{}{
		"id":        string(message.ID),
		"topic":     string(message.Topic),
		"payload":   message.Payload,
		"timestamp": message.Timestamp,
		"data":      messageData,
	}

	_, err = d.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to store message in stream: %w", err)
	}

	// Update topic metadata
	d.updateTopicMetadata(ctx, message.Topic)

	return nil
}

// StoreBatch implements MessageStore.StoreBatch
func (d *DragonflyStorage) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	pipe := d.client.Pipeline()

	for _, message := range batch.Messages {
		messageData, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		streamKey := fmt.Sprintf("STREAM_%s", message.Topic)
		values := map[string]interface{}{
			"id":        string(message.ID),
			"topic":     string(message.Topic),
			"payload":   message.Payload,
			"timestamp": message.Timestamp,
			"data":      messageData,
		}

		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: streamKey,
			Values: values,
		})
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}

	return nil
}

// Fetch implements MessageStore.Fetch
func (d *DragonflyStorage) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	streamKey := fmt.Sprintf("STREAM_%s", topic)

	// Convert offset to Redis stream ID
	start := fmt.Sprintf("%d-0", offset)
	if offset == 0 {
		start = "0" // From beginning
	}

	result, err := d.client.XRevRange(ctx, streamKey, "+", start).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	// Limit results
	if len(result) > limit {
		result = result[:limit]
	}

	messages := make([]*types.PortaskMessage, 0, len(result))
	for _, msg := range result {
		if dataStr, ok := msg.Values["data"].(string); ok {
			var message types.PortaskMessage
			if err := json.Unmarshal([]byte(dataStr), &message); err == nil {
				messages = append(messages, &message)
			}
		}
	}

	return messages, nil
}

// FetchByID implements MessageStore.FetchByID
func (d *DragonflyStorage) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	// Search across all streams (this is expensive - in production we'd maintain an index)
	keys := d.client.Keys(ctx, "STREAM_*").Val()

	for _, streamKey := range keys {
		result, err := d.client.XRange(ctx, streamKey, "-", "+").Result()
		if err != nil {
			continue
		}

		for _, msg := range result {
			if idStr, ok := msg.Values["id"].(string); ok && idStr == string(messageID) {
				if dataStr, ok := msg.Values["data"].(string); ok {
					var message types.PortaskMessage
					if err := json.Unmarshal([]byte(dataStr), &message); err == nil {
						return &message, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("message not found: %s", messageID)
}

// Delete implements MessageStore.Delete
func (d *DragonflyStorage) Delete(ctx context.Context, messageID types.MessageID) error {
	// Find and delete from appropriate stream
	keys := d.client.Keys(ctx, "STREAM_*").Val()

	for _, streamKey := range keys {
		result, err := d.client.XRange(ctx, streamKey, "-", "+").Result()
		if err != nil {
			continue
		}

		for _, msg := range result {
			if idStr, ok := msg.Values["id"].(string); ok && idStr == string(messageID) {
				return d.client.XDel(ctx, streamKey, msg.ID).Err()
			}
		}
	}

	return fmt.Errorf("message not found for deletion: %s", messageID)
}

// DeleteBatch implements MessageStore.DeleteBatch
func (d *DragonflyStorage) DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error {
	for _, id := range messageIDs {
		if err := d.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// CreateTopic implements MessageStore.CreateTopic
func (d *DragonflyStorage) CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error {
	topicData, err := json.Marshal(topicInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal topic info: %w", err)
	}

	key := fmt.Sprintf("TOPIC_META_%s", topicInfo.Name)
	return d.client.Set(ctx, key, topicData, 0).Err()
}

// DeleteTopic implements MessageStore.DeleteTopic
func (d *DragonflyStorage) DeleteTopic(ctx context.Context, topic types.TopicName) error {
	// Delete stream
	streamKey := fmt.Sprintf("STREAM_%s", topic)
	d.client.Del(ctx, streamKey)

	// Delete metadata
	metaKey := fmt.Sprintf("TOPIC_META_%s", topic)
	return d.client.Del(ctx, metaKey).Err()
}

// GetTopicInfo implements MessageStore.GetTopicInfo
func (d *DragonflyStorage) GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error) {
	key := fmt.Sprintf("TOPIC_META_%s", topic)
	data, err := d.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("topic not found: %s", topic)
		}
		return nil, err
	}

	var topicInfo types.TopicInfo
	err = json.Unmarshal([]byte(data), &topicInfo)
	return &topicInfo, err
}

// ListTopics implements MessageStore.ListTopics
func (d *DragonflyStorage) ListTopics(ctx context.Context) ([]*types.TopicInfo, error) {
	keys := d.client.Keys(ctx, "TOPIC_META_*").Val()
	topics := make([]*types.TopicInfo, 0, len(keys))

	for _, key := range keys {
		data, err := d.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var topicInfo types.TopicInfo
		if err := json.Unmarshal([]byte(data), &topicInfo); err == nil {
			topics = append(topics, &topicInfo)
		}
	}

	return topics, nil
}

// TopicExists implements MessageStore.TopicExists
func (d *DragonflyStorage) TopicExists(ctx context.Context, topic types.TopicName) (bool, error) {
	key := fmt.Sprintf("TOPIC_META_%s", topic)
	exists, err := d.client.Exists(ctx, key).Result()
	return exists > 0, err
}

// GetPartitionInfo implements MessageStore.GetPartitionInfo
func (d *DragonflyStorage) GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error) {
	// Simplified partition info
	return &types.PartitionInfo{
		Topic:     topic,
		Partition: partition,
		Leader:    "node-0", // Single node for now
		Replicas:  []string{"node-0"},
		ISR:       []string{"node-0"},
	}, nil
}

// GetPartitionCount implements MessageStore.GetPartitionCount
func (d *DragonflyStorage) GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error) {
	// For now, return 1 partition per topic
	return 1, nil
}

// GetLatestOffset implements MessageStore.GetLatestOffset
func (d *DragonflyStorage) GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	streamKey := fmt.Sprintf("STREAM_%s", topic)
	result, err := d.client.XLen(ctx, streamKey).Result()
	return result, err
}

// GetEarliestOffset implements MessageStore.GetEarliestOffset
func (d *DragonflyStorage) GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	// For Redis streams, earliest is always 0
	return 0, nil
}

// CommitOffset implements MessageStore.CommitOffset
func (d *DragonflyStorage) CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error {
	key := fmt.Sprintf("OFFSET_%s_%s_%d", offset.ConsumerID, offset.Topic, offset.Partition)
	offsetData, err := json.Marshal(offset)
	if err != nil {
		return err
	}
	return d.client.Set(ctx, key, offsetData, 0).Err()
}

// CommitOffsetBatch implements MessageStore.CommitOffsetBatch
func (d *DragonflyStorage) CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error {
	pipe := d.client.Pipeline()

	for _, offset := range offsets {
		key := fmt.Sprintf("OFFSET_%s_%s_%d", offset.ConsumerID, offset.Topic, offset.Partition)
		offsetData, err := json.Marshal(offset)
		if err != nil {
			return err
		}
		pipe.Set(ctx, key, offsetData, 0)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetOffset implements MessageStore.GetOffset
func (d *DragonflyStorage) GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error) {
	key := fmt.Sprintf("OFFSET_%s_%s_%d", consumerID, topic, partition)
	data, err := d.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return &types.ConsumerOffset{
				ConsumerID: consumerID,
				Topic:      topic,
				Partition:  partition,
				Offset:     0,
			}, nil
		}
		return nil, err
	}

	var offset types.ConsumerOffset
	err = json.Unmarshal([]byte(data), &offset)
	return &offset, err
}

// GetConsumerOffsets implements MessageStore.GetConsumerOffsets
func (d *DragonflyStorage) GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error) {
	pattern := fmt.Sprintf("OFFSET_%s_*", consumerID)
	keys := d.client.Keys(ctx, pattern).Val()

	offsets := make([]*types.ConsumerOffset, 0, len(keys))
	for _, key := range keys {
		data, err := d.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var offset types.ConsumerOffset
		if err := json.Unmarshal([]byte(data), &offset); err == nil {
			offsets = append(offsets, &offset)
		}
	}

	return offsets, nil
}

// ListConsumers implements MessageStore.ListConsumers
func (d *DragonflyStorage) ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error) {
	pattern := fmt.Sprintf("OFFSET_*_%s_*", topic)
	keys := d.client.Keys(ctx, pattern).Val()

	consumerMap := make(map[types.ConsumerID]bool)
	for _, key := range keys {
		// Extract consumer ID from key: OFFSET_<consumerID>_<topic>_<partition>
		parts := []rune(key)
		start := len("OFFSET_")
		end := start
		for i := start; i < len(parts); i++ {
			if parts[i] == '_' {
				// Check if this is the topic separator
				remaining := string(parts[i+1:])
				if len(remaining) > len(topic) && remaining[:len(topic)] == string(topic) {
					end = i
					break
				}
			}
		}

		if end > start {
			consumerID := types.ConsumerID(parts[start:end])
			consumerMap[consumerID] = true
		}
	}

	consumers := make([]types.ConsumerID, 0, len(consumerMap))
	for consumerID := range consumerMap {
		consumers = append(consumers, consumerID)
	}

	return consumers, nil
}

// Ping implements MessageStore.Ping
func (d *DragonflyStorage) Ping(ctx context.Context) error {
	return d.client.Ping(ctx).Err()
}

// Stats implements MessageStore.Stats
func (d *DragonflyStorage) Stats(ctx context.Context) (*StorageStats, error) {
	_ = d.client.Info(ctx, "memory", "stats").Val()

	// Parse basic stats (simplified)
	stats := &StorageStats{
		ActiveConnections: 1, // Simplified
		Status:            "connected",
		LastHealthCheck:   time.Now(),
	}

	return stats, nil
}

// Cleanup implements MessageStore.Cleanup
func (d *DragonflyStorage) Cleanup(ctx context.Context, retentionPolicy *RetentionPolicy) error {
	if retentionPolicy.MaxAge > 0 {
		cutoff := time.Now().Add(-retentionPolicy.MaxAge).Unix()

		// Clean old messages from streams
		keys := d.client.Keys(ctx, "STREAM_*").Val()
		for _, streamKey := range keys {
			// Remove messages older than cutoff
			d.client.XTrimMinID(ctx, streamKey, fmt.Sprintf("%d-0", cutoff))
		}
	}

	return nil
}

// Close implements MessageStore.Close
func (d *DragonflyStorage) Close() error {
	return d.client.Close()
}

// updateTopicMetadata updates topic metadata
func (d *DragonflyStorage) updateTopicMetadata(ctx context.Context, topic types.TopicName) {
	key := fmt.Sprintf("TOPIC_META_%s", topic)
	exists, _ := d.client.Exists(ctx, key).Result()

	if exists == 0 {
		// Create default topic info
		topicInfo := &types.TopicInfo{
			Name:              topic,
			Partitions:        1,
			ReplicationFactor: 1,
			CreatedAt:         time.Now().Unix(),
		}

		data, _ := json.Marshal(topicInfo)
		d.client.Set(ctx, key, data, 0)
	}
}
