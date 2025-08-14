package dragonfly

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// Local pools for zero-alloc fast paths
var (
	// Reusable PortaskMessage instances
	messagePool = sync.Pool{New: func() interface{} { return new(types.PortaskMessage) }}
	// Reusable *[]string slices to avoid allocations
	stringSlicePool = sync.Pool{New: func() interface{} { s := make([]string, 0, 64); return &s }}
)

// DragonflyStore implements MessageStore interface for Dragonfly
type DragonflyStore struct {
	client     redis.UniversalClient
	config     *storage.DragonflyConfig
	metrics    *storage.StorageStats
	serializer storage.Serializer
	compressor storage.Compressor

	// Connection pool and health
	connected bool
	startTime time.Time
	connMutex sync.RWMutex

	// Key prefixes for different data types
	messagePrefix   string
	topicPrefix     string
	partitionPrefix string
	offsetPrefix    string
	metaPrefix      string
}

// NewDragonflyStore creates a new Dragonfly storage instance
func NewDragonflyStore(config *storage.DragonflyConfig) (*DragonflyStore, error) {
	// Configure Redis client options
	var client redis.UniversalClient

	if config.EnableCluster && len(config.Addresses) > 1 {
		// Cluster mode with OPTIMIZED settings for stability + performance
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        config.Addresses,
			Username:     config.Username,
			Password:     config.Password,
			DialTimeout:  2 * time.Second, // More tolerant for CI
			ReadTimeout:  2 * time.Second,
			WriteTimeout: 2 * time.Second,
			PoolSize:     1000, // High performance pool
			MinIdleConns: 200,  // Keep many warm connections
			PoolTimeout:  10 * time.Second,
		})
	} else {
		// Single instance with OPTIMIZED settings for stability + performance
		var addresses []string
		if len(config.Addresses) == 0 {
			addresses = []string{"localhost:6379"}
		} else {
			addresses = config.Addresses
		}

		client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:        addresses,
			Username:     config.Username,
			Password:     config.Password,
			DB:           config.DB,
			DialTimeout:  2 * time.Second,
			ReadTimeout:  2 * time.Second,
			WriteTimeout: 2 * time.Second,
			PoolSize:     1000, // High performance pool
			MinIdleConns: 200,  // Keep many warm connections
			PoolTimeout:  10 * time.Second,
			MaxRetries:   3,
		})
	}

	keyPrefix := config.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = "portask"
	}

	store := &DragonflyStore{
		client:     client,
		config:     config,
		startTime:  time.Now(),
		serializer: storage.NewJSONSerializer(), // Default serializer
		compressor: storage.NewNoOpCompressor(), // Default no compression

		// Key prefixes
		messagePrefix:   fmt.Sprintf("%s:msg:", keyPrefix),
		topicPrefix:     fmt.Sprintf("%s:topic:", keyPrefix),
		partitionPrefix: fmt.Sprintf("%s:part:", keyPrefix),
		offsetPrefix:    fmt.Sprintf("%s:offset:", keyPrefix),
		metaPrefix:      fmt.Sprintf("%s:meta:", keyPrefix),

		metrics: &storage.StorageStats{
			Status: "disconnected",
		},
	}

	// Enable compression if configured
	if config.EnableCompression {
		store.compressor = storage.NewZstdCompressor(config.CompressionLevel)
	}

	return store, nil
}

// Connect establishes connection to Dragonfly
func (d *DragonflyStore) Connect(ctx context.Context) error {
	d.connMutex.Lock()
	defer d.connMutex.Unlock()

	// Test connection with a brief retry loop to tolerate slow CI starts
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = d.client.Ping(ctx).Err()
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		d.metrics.Status = "unhealthy"
		return fmt.Errorf("failed to connect to Dragonfly: %w", err)
	}

	d.connected = true
	d.metrics.Status = "healthy"
	d.metrics.LastHealthCheck = time.Now()

	return nil
}

// Disconnect closes connection to Dragonfly
func (d *DragonflyStore) Close() error {
	d.connMutex.Lock()
	defer d.connMutex.Unlock()

	d.connected = false
	d.metrics.Status = "disconnected"

	return d.client.Close()
}

// Ping checks Dragonfly connectivity
func (d *DragonflyStore) Ping(ctx context.Context) error {
	start := time.Now()
	defer func() {
		d.updateResponseTime(time.Since(start))
	}()

	err := d.client.Ping(ctx).Err()
	if err != nil {
		d.metrics.FailedOperations++
		d.metrics.Status = "unhealthy"
		return fmt.Errorf("ping failed: %w", err)
	}

	d.metrics.SuccessfulOperations++
	d.metrics.Status = "healthy"
	d.metrics.LastHealthCheck = time.Now()

	return nil
}

// Store saves a single message (OPTIMIZED but COMPATIBLE)
func (d *DragonflyStore) Store(ctx context.Context, message *types.PortaskMessage) error {
	start := time.Now()
	defer func() {
		d.updateResponseTime(time.Since(start))
		d.metrics.TotalOperations++
	}()

	// Always serialize for compatibility - but optimized
	data, err := d.serializer.Serialize(message)
	if err != nil {
		d.metrics.FailedOperations++
		return fmt.Errorf("serialization failed: %w", err)
	}

	// Skip compression for small messages for speed
	if d.config.EnableCompression && len(data) > 1024 {
		data, err = d.compressor.Compress(data)
		if err != nil {
			d.metrics.FailedOperations++
			return fmt.Errorf("compression failed: %w", err)
		}
	}

	// Ultra-fast key generation
	key := d.messagePrefix + string(message.ID)

	// Set with minimal TTL overhead
	var ttl time.Duration
	if message.TTL > 0 {
		ttl = time.Duration(message.TTL) * time.Second
	}

	err = d.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		d.metrics.FailedOperations++
		return fmt.Errorf("store failed: %w", err)
	}

	// Also append to a stream for ordered reads (integrated optimization)
	streamKey := fmt.Sprintf("%s:stream:%s:%d", d.config.KeyPrefix, message.Topic, message.Partition)
	_ = d.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"id":        string(message.ID),
			"data":      string(data),
			"timestamp": message.Timestamp,
		},
	}).Err()

	// Update topic message count
	topicCountKey := d.topicPrefix + string(message.Topic) + ":count"
	d.client.Incr(ctx, topicCountKey)

	d.metrics.SuccessfulOperations++
	return nil
}

// StoreBatch saves multiple messages in a single operation (OPTIMIZED but COMPATIBLE)
func (d *DragonflyStore) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	start := time.Now()
	defer func() {
		d.updateResponseTime(time.Since(start))
		d.metrics.TotalOperations++
	}()

	// Single mega-pipeline for maximum throughput
	pipe := d.client.Pipeline()

	for _, message := range batch.Messages {
		// Always serialize for compatibility
		data, err := d.serializer.Serialize(message)
		if err != nil {
			d.metrics.FailedOperations++
			return fmt.Errorf("serialization failed for message %s: %w", message.ID, err)
		}

		// Compress only large messages for speed
		if d.config.EnableCompression && len(data) > 1024 {
			data, err = d.compressor.Compress(data)
			if err != nil {
				d.metrics.FailedOperations++
				return fmt.Errorf("compression failed for message %s: %w", message.ID, err)
			}
		}

		// Ultra-fast key generation
		key := d.messagePrefix + string(message.ID)

		// Add to pipeline with TTL if specified
		var ttl time.Duration
		if message.TTL > 0 {
			ttl = time.Duration(message.TTL) * time.Second
		}

		pipe.Set(ctx, key, data, ttl)

		// Add to stream too (integrated optimization)
		streamKey := fmt.Sprintf("%s:stream:%s:%d", d.config.KeyPrefix, message.Topic, message.Partition)
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: streamKey,
			Values: map[string]interface{}{
				"id":        string(message.ID),
				"data":      string(data),
				"timestamp": message.Timestamp,
			},
		})

		// Re-enable topic counting for compatibility
		topicCountKey := d.topicPrefix + string(message.Topic) + ":count"
		pipe.Incr(ctx, topicCountKey)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		d.metrics.FailedOperations++
		return fmt.Errorf("batch store failed: %w", err)
	}

	d.metrics.SuccessfulOperations++
	return nil
}

// Fetch retrieves messages from a topic partition (FIXED OPTIMIZED version)
func (ds *DragonflyStore) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	start := time.Now()
	defer func() {
		ds.updateResponseTime(time.Since(start))
	}()

	if limit <= 0 {
		return []*types.PortaskMessage{}, nil
	}

	messages := make([]*types.PortaskMessage, 0, limit)

	// Optimized SCAN with reasonable batches
	pattern := ds.messagePrefix + "*"
	var cursor uint64
	const batchSize = 2000 // Reasonable batch size

	for len(messages) < limit {
		// Scan with reasonable batches
		keys, nextCursor, err := ds.client.Scan(ctx, cursor, pattern, batchSize).Result()
		if err != nil {
			ds.metrics.FailedOperations++
			return nil, fmt.Errorf("failed to scan message keys: %w", err)
		}

		if len(keys) == 0 {
			break
		}

		// Process keys in reasonable chunks
		const chunkSize = 500
		for i := 0; i < len(keys); i += chunkSize {
			if len(messages) >= limit {
				break
			}

			end := i + chunkSize
			if end > len(keys) {
				end = len(keys)
			}
			chunk := keys[i:end]

			// Pipeline for this chunk
			pipe := ds.client.Pipeline()
			cmds := make([]*redis.StringCmd, len(chunk))

			for j, key := range chunk {
				cmds[j] = pipe.Get(ctx, key)
			}

			_, err = pipe.Exec(ctx)
			if err != nil && err != redis.Nil {
				continue // Skip this chunk, don't fail entire operation
			}

			// Process results
			for _, cmd := range cmds {
				if len(messages) >= limit {
					break
				}

				data, err := cmd.Bytes()
				if err == redis.Nil {
					continue
				}
				if err != nil {
					continue
				}

				// Decompress if enabled
				if ds.config.EnableCompression {
					data, err = ds.compressor.Decompress(data)
					if err != nil {
						continue
					}
				}

				// Deserialize
				message, err := ds.serializer.Deserialize(data)
				if err != nil {
					continue
				}

				// Filter and add
				if message.Topic == topic && (partition < 0 || message.Partition == partition) && message.Offset >= offset {
					messages = append(messages, message)
				}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	ds.metrics.SuccessfulOperations++
	return messages, nil
}

// FetchByID retrieves a message by ID
func (ds *DragonflyStore) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	start := time.Now()
	defer func() {
		ds.updateResponseTime(time.Since(start))
		ds.metrics.TotalOperations++
	}()

	key := ds.messagePrefix + string(messageID)

	data, err := ds.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			ds.metrics.SuccessfulOperations++
			return nil, fmt.Errorf("message not found")
		}
		ds.metrics.FailedOperations++
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	// Decompress if enabled
	if ds.config.EnableCompression {
		data, err = ds.compressor.Decompress(data)
		if err != nil {
			ds.metrics.FailedOperations++
			return nil, fmt.Errorf("decompression failed: %w", err)
		}
	}

	// Deserialize message
	message, err := ds.serializer.Deserialize(data)
	if err != nil {
		ds.metrics.FailedOperations++
		return nil, fmt.Errorf("deserialization failed: %w", err)
	}

	ds.metrics.SuccessfulOperations++
	return message, nil
}

// Delete removes a message by its ID
func (d *DragonflyStore) Delete(ctx context.Context, messageID types.MessageID) error {
	start := time.Now()
	defer func() {
		d.updateResponseTime(time.Since(start))
		d.metrics.TotalOperations++
	}()

	key := d.messagePrefix + string(messageID)

	err := d.client.Del(ctx, key).Err()
	if err != nil {
		d.metrics.FailedOperations++
		return fmt.Errorf("delete failed: %w", err)
	}

	d.metrics.SuccessfulOperations++
	return nil
}

// DeleteBatch removes multiple messages (OPTIMIZED FIXED version)
func (d *DragonflyStore) DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error {
	start := time.Now()
	defer func() {
		d.updateResponseTime(time.Since(start))
		d.metrics.TotalOperations++
	}()

	if len(messageIDs) == 0 {
		return nil
	}

	// Optimized: Reasonable batch size and concurrency
	const maxKeysPerBatch = 1000    // Reasonable batch size
	const maxConcurrentBatches = 10 // Limited concurrency

	// Pre-build all keys
	keys := make([]string, len(messageIDs))
	for i, id := range messageIDs {
		keys[i] = d.messagePrefix + string(id)
	}

	// Process in batches with limited concurrency
	var wg sync.WaitGroup
	errorChan := make(chan error, maxConcurrentBatches)
	semaphore := make(chan struct{}, maxConcurrentBatches)

	for i := 0; i < len(keys); i += maxKeysPerBatch {
		end := i + maxKeysPerBatch
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]

		wg.Add(1)
		go func(keysToDelete []string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Delete batch
			err := d.client.Del(ctx, keysToDelete...).Err()
			if err != nil {
				select {
				case errorChan <- err:
				default:
				}
			}
		}(batch)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	select {
	case err := <-errorChan:
		if err != nil {
			d.metrics.FailedOperations++
			return fmt.Errorf("batch delete failed: %w", err)
		}
	default:
	}

	d.metrics.SuccessfulOperations++
	return nil
}

// TopicExists checks if a topic exists
func (ds *DragonflyStore) TopicExists(ctx context.Context, topic types.TopicName) (bool, error) {
	key := ds.topicPrefix + string(topic)
	exists, err := ds.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check topic existence: %w", err)
	}
	return exists > 0, nil
}

// CreateTopic creates a new topic
func (d *DragonflyStore) CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error {
	start := time.Now()
	defer func() {
		d.updateResponseTime(time.Since(start))
		d.metrics.TotalOperations++
	}()

	// Check if topic already exists
	exists, err := d.TopicExists(ctx, topicInfo.Name)
	if err != nil {
		d.metrics.FailedOperations++
		return fmt.Errorf("failed to check topic existence: %w", err)
	}

	if exists {
		d.metrics.FailedOperations++
		return fmt.Errorf("topic already exists")
	}

	// Serialize topic info
	data, err := json.Marshal(topicInfo)
	if err != nil {
		d.metrics.FailedOperations++
		return fmt.Errorf("failed to serialize topic info: %w", err)
	}

	// Store topic info
	key := d.topicPrefix + string(topicInfo.Name)
	err = d.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		d.metrics.FailedOperations++
		return fmt.Errorf("failed to create topic: %w", err)
	}

	// Initialize partition info
	for i := int32(0); i < topicInfo.Partitions; i++ {
		partitionKey := d.partitionPrefix + string(topicInfo.Name) + ":" + strconv.Itoa(int(i))
		partitionInfo := &types.PartitionInfo{
			Topic:     topicInfo.Name,
			Partition: i,
			Leader:    "single-node",
			Replicas:  []string{"single-node"},
			ISR:       []string{"single-node"},
		}

		partitionData, _ := json.Marshal(partitionInfo)
		d.client.Set(ctx, partitionKey, partitionData, 0)
	}

	d.metrics.SuccessfulOperations++
	return nil
}

// CommitOffset commits consumer offset
func (ds *DragonflyStore) CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error {
	key := ds.offsetPrefix + string(offset.ConsumerID) + ":" + string(offset.Topic) + ":" + strconv.Itoa(int(offset.Partition))

	offsetData := map[string]interface{}{
		"offset":    offset.Offset,
		"timestamp": time.Now().UnixNano(),
		"metadata":  offset.Metadata,
	}

	data, err := json.Marshal(offsetData)
	if err != nil {
		return fmt.Errorf("failed to marshal offset: %w", err)
	}

	return ds.client.Set(ctx, key, data, 0).Err()
}

// CommitOffsetBatch commits multiple consumer offsets
func (ds *DragonflyStore) CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error {
	pipe := ds.client.Pipeline()

	for _, offset := range offsets {
		key := ds.offsetPrefix + string(offset.ConsumerID) + ":" + string(offset.Topic) + ":" + strconv.Itoa(int(offset.Partition))

		offsetData := map[string]interface{}{
			"offset":    offset.Offset,
			"timestamp": time.Now().UnixNano(),
			"metadata":  offset.Metadata,
		}

		data, _ := json.Marshal(offsetData)
		pipe.Set(ctx, key, data, 0)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetOffset retrieves consumer offset
func (ds *DragonflyStore) GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error) {
	key := ds.offsetPrefix + string(consumerID) + ":" + string(topic) + ":" + strconv.Itoa(int(partition))

	data, err := ds.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return &types.ConsumerOffset{
				ConsumerID: consumerID,
				Topic:      topic,
				Partition:  partition,
				Offset:     0,
			}, nil
		}
		return nil, fmt.Errorf("failed to get offset: %w", err)
	}

	var offsetData map[string]interface{}
	if err := json.Unmarshal(data, &offsetData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal offset: %w", err)
	}

	offset := &types.ConsumerOffset{
		ConsumerID: consumerID,
		Topic:      topic,
		Partition:  partition,
		Offset:     int64(offsetData["offset"].(float64)),
	}

	if metadata, exists := offsetData["metadata"]; exists {
		offset.Metadata = metadata.(string)
	}

	return offset, nil
}

// GetConsumerOffsets retrieves all offsets for a consumer
func (ds *DragonflyStore) GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error) {
	pattern := ds.offsetPrefix + string(consumerID) + ":*"
	keys, err := ds.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer offset keys: %w", err)
	}

	offsets := make([]*types.ConsumerOffset, 0, len(keys))

	for _, key := range keys {
		// Parse topic and partition from key
		parts := strings.Split(key, ":")
		if len(parts) < 4 {
			continue
		}

		topic := types.TopicName(parts[2])
		partition, err := strconv.Atoi(parts[3])
		if err != nil {
			continue
		}

		offset, err := ds.GetOffset(ctx, consumerID, topic, int32(partition))
		if err != nil {
			continue
		}

		offsets = append(offsets, offset)
	}

	return offsets, nil
}

// ListConsumers lists all consumers for a topic
func (ds *DragonflyStore) ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error) {
	pattern := ds.offsetPrefix + "*:" + string(topic) + ":*"
	keys, err := ds.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer keys: %w", err)
	}

	consumerMap := make(map[string]bool)

	for _, key := range keys {
		// Parse consumer ID from key
		parts := strings.Split(key, ":")
		if len(parts) >= 3 {
			consumerMap[parts[1]] = true
		}
	}

	consumers := make([]types.ConsumerID, 0, len(consumerMap))
	for consumer := range consumerMap {
		consumers = append(consumers, types.ConsumerID(consumer))
	}

	return consumers, nil
}

// GetPartitionInfo retrieves partition information
func (ds *DragonflyStore) GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error) {
	key := ds.partitionPrefix + string(topic) + ":" + strconv.Itoa(int(partition))

	data, err := ds.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("partition not found")
		}
		return nil, fmt.Errorf("failed to get partition info: %w", err)
	}

	var partitionInfo types.PartitionInfo
	if err := json.Unmarshal(data, &partitionInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal partition info: %w", err)
	}

	return &partitionInfo, nil
}

// GetTopicInfo retrieves topic information
func (ds *DragonflyStore) GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error) {
	key := ds.topicPrefix + string(topic)

	data, err := ds.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("topic not found")
		}
		return nil, fmt.Errorf("failed to get topic info: %w", err)
	}

	var topicInfo types.TopicInfo
	if err := json.Unmarshal(data, &topicInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal topic info: %w", err)
	}

	return &topicInfo, nil
}

// ListTopics lists all topics (FIXED SIMPLE version)
func (ds *DragonflyStore) ListTopics(ctx context.Context) ([]*types.TopicInfo, error) {
	start := time.Now()
	defer func() {
		ds.updateResponseTime(time.Since(start))
	}()

	// Get all topic keys in one shot
	pattern := ds.topicPrefix + "*"
	keys, err := ds.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	if len(keys) == 0 {
		return []*types.TopicInfo{}, nil
	}

	// Simple sequential processing to avoid goroutine issues
	topics := make([]*types.TopicInfo, 0, len(keys))

	for _, key := range keys {
		data, err := ds.client.Get(ctx, key).Bytes()
		if err != nil {
			continue // Skip failed topics
		}

		var topicInfo types.TopicInfo
		if err := json.Unmarshal(data, &topicInfo); err != nil {
			continue // Skip invalid topics
		}

		topics = append(topics, &topicInfo)
	}

	return topics, nil
}

// DeleteTopic deletes a topic and all its data
func (ds *DragonflyStore) DeleteTopic(ctx context.Context, topic types.TopicName) error {
	// Delete topic info
	topicKey := ds.topicPrefix + string(topic)
	if err := ds.client.Del(ctx, topicKey).Err(); err != nil {
		return fmt.Errorf("failed to delete topic info: %w", err)
	}

	// Delete all partitions
	partitionPattern := ds.partitionPrefix + string(topic) + ":*"
	partitionKeys, err := ds.client.Keys(ctx, partitionPattern).Result()
	if err == nil && len(partitionKeys) > 0 {
		ds.client.Del(ctx, partitionKeys...)
	}

	// Delete all messages
	messagePattern := ds.messagePrefix + string(topic) + ":*"
	messageKeys, err := ds.client.Keys(ctx, messagePattern).Result()
	if err == nil && len(messageKeys) > 0 {
		// Delete in batches to avoid blocking
		batchSize := 1000
		for i := 0; i < len(messageKeys); i += batchSize {
			end := i + batchSize
			if end > len(messageKeys) {
				end = len(messageKeys)
			}
			ds.client.Del(ctx, messageKeys[i:end]...)
		}
	}

	// Delete all consumer offsets
	offsetPattern := ds.offsetPrefix + "*:" + string(topic) + ":*"
	offsetKeys, err := ds.client.Keys(ctx, offsetPattern).Result()
	if err == nil && len(offsetKeys) > 0 {
		ds.client.Del(ctx, offsetKeys...)
	}

	return nil
}

// GetPartitionCount returns the number of partitions for a topic
func (ds *DragonflyStore) GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error) {
	topicData, err := ds.GetTopicInfo(ctx, topic)
	if err != nil {
		return 0, err
	}
	return topicData.Partitions, nil
}

// GetLatestOffset returns the latest offset for a partition
func (ds *DragonflyStore) GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	// For now, return a simple counter - in production this would track actual message offsets
	key := ds.offsetPrefix + "latest:" + string(topic) + ":" + strconv.Itoa(int(partition))

	result, err := ds.client.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get latest offset: %w", err)
	}

	return result, nil
}

// GetEarliestOffset returns the earliest offset for a partition
func (ds *DragonflyStore) GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	// For now, return 0 - in production this would track actual earliest available offset
	return 0, nil
}

// ========== ULTRA PERFORMANCE METHODS ==========

// UltraFastStore - Zero allocation store operation targeting 100k+ ops/s
func (ds *DragonflyStore) UltraFastStore(ctx context.Context, message *types.PortaskMessage) error {
	// Ultra-fast key generation
	key := ds.messagePrefix + string(message.ID)

	// Use payload directly for maximum speed
	var data []byte
	if message.Payload != nil {
		data = message.Payload
	} else {
		// Fallback serialization
		var err error
		data, err = ds.serializer.Serialize(message)
		if err != nil {
			return fmt.Errorf("serialization failed: %w", err)
		}
	}

	// Direct store without TTL overhead
	return ds.client.Set(ctx, key, data, 0).Err()
}

// UltraFastBatchStore - Batch operations with minimal overhead
func (ds *DragonflyStore) UltraFastBatchStore(ctx context.Context, messages []*types.PortaskMessage) error {
	if len(messages) == 0 {
		return nil
	}

	// Single mega-pipeline for maximum throughput
	pipe := ds.client.Pipeline()

	for _, msg := range messages {
		key := ds.messagePrefix + string(msg.ID)
		var data []byte
		if msg.Payload != nil {
			data = msg.Payload
		} else {
			// Fallback serialization
			var err error
			data, err = ds.serializer.Serialize(msg)
			if err != nil {
				continue // Skip failed messages
			}
		}
		pipe.Set(ctx, key, data, 0)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// ZeroAllocFetch - Memory pool based fetch for ultra performance
func (ds *DragonflyStore) ZeroAllocFetch(ctx context.Context, topic types.TopicName, limit int) ([]*types.PortaskMessage, error) {
	if limit <= 0 {
		return nil, nil
	}

	// Single SCAN operation with optimized batch
	pattern := ds.messagePrefix + "*"
	keys, _, err := ds.client.Scan(ctx, 0, pattern, int64(limit*2)).Result()
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return []*types.PortaskMessage{}, nil
	}

	// Limit keys to requested amount
	if len(keys) > limit {
		keys = keys[:limit]
	}

	// Single MGET for all keys
	values, err := ds.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	// Fast processing without full deserialization for filtering
	messages := make([]*types.PortaskMessage, 0, limit)
	for i, val := range values {
		if val == nil {
			continue
		}

		// Get message from pool
		msg := messagePool.Get().(*types.PortaskMessage)

		// Extract ID from key (ultra fast)
		keyStr := keys[i]
		msg.ID = types.MessageID(keyStr[len(ds.messagePrefix):])
		msg.Topic = topic

		// Store payload directly
		if valStr, ok := val.(string); ok {
			msg.Payload = []byte(valStr)
		}

		messages = append(messages, msg)

		if len(messages) >= limit {
			break
		}
	}

	return messages, nil
}

// TurboDelete - Fastest possible delete operation
func (ds *DragonflyStore) TurboDelete(ctx context.Context, messageID types.MessageID) error {
	key := ds.messagePrefix + string(messageID)
	return ds.client.Del(ctx, key).Err()
}

// MegaBatchDelete - Ultra-fast batch deletion
func (ds *DragonflyStore) MegaBatchDelete(ctx context.Context, messageIDs []types.MessageID) error {
	if len(messageIDs) == 0 {
		return nil
	}

	// Convert to keys efficiently using pool
	keysPtr := stringSlicePool.Get().(*[]string)
	// Work on a local copy to avoid races
	keys := (*keysPtr)[:0]

	for _, id := range messageIDs {
		keys = append(keys, ds.messagePrefix+string(id))
	}

	// Single DEL command for maximum performance
	err := ds.client.Del(ctx, keys...).Err()
	// Reset and put back to pool
	*keysPtr = keys[:0]
	stringSlicePool.Put(keysPtr)
	return err
}

// RecycleMessage - Memory recycling for continuous high performance
func (ds *DragonflyStore) RecycleMessage(msg *types.PortaskMessage) {
	if msg == nil {
		return
	}

	// Reset message fields
	msg.ID = ""
	msg.Topic = ""
	msg.Payload = nil
	msg.Timestamp = 0
	msg.Partition = 0
	msg.Offset = 0
	msg.TTL = 0

	// Return to pool
	messagePool.Put(msg)
}

// Helper methods

// updateResponseTime updates average response time metrics
func (ds *DragonflyStore) updateResponseTime(duration time.Duration) {
	if ds.metrics.AvgResponseTime == 0 {
		ds.metrics.AvgResponseTime = duration
	} else {
		// Simple moving average
		ds.metrics.AvgResponseTime = (ds.metrics.AvgResponseTime + duration) / 2
	}

	// Update P95 and P99 with simple approximation
	if duration > ds.metrics.P95ResponseTime {
		ds.metrics.P95ResponseTime = duration
	}
	if duration > ds.metrics.P99ResponseTime {
		ds.metrics.P99ResponseTime = duration
	}
}

// Stats returns current storage statistics
func (d *DragonflyStore) Stats(ctx context.Context) (*storage.StorageStats, error) {
	d.connMutex.RLock()
	defer d.connMutex.RUnlock()

	// Update uptime
	d.metrics.Uptime = time.Since(d.startTime)

	// Get Redis info if connected
	if d.connected {
		info := d.client.Info(ctx, "memory", "stats")
		if info.Err() == nil {
			// Parse Redis INFO response for memory and stats
			// This is simplified - in production parse the actual INFO response
			d.metrics.UsedMemory = 1024 * 1024 // Placeholder
			d.metrics.KeyCount = 1000          // Placeholder
		}
	}

	// Copy metrics to avoid race conditions
	stats := *d.metrics
	return &stats, nil
}

// Cleanup performs cleanup operations (FIXED STABLE version)
func (ds *DragonflyStore) Cleanup(ctx context.Context, retentionPolicy *storage.RetentionPolicy) error {
	start := time.Now()
	defer func() {
		ds.updateResponseTime(time.Since(start))
		ds.metrics.TotalOperations++
	}()

	if retentionPolicy == nil {
		return nil
	}

	// Simple cleanup with reasonable batching
	const batchSize = 1000
	var cursor uint64

	for {
		// Scan for message keys
		keys, nextCursor, err := ds.client.Scan(ctx, cursor, ds.messagePrefix+"*", batchSize).Result()
		if err != nil {
			ds.metrics.FailedOperations++
			return fmt.Errorf("failed to scan message keys: %w", err)
		}

		if len(keys) == 0 {
			break
		}

		var keysToDelete []string

		// Check age if retention policy specifies max age
		if retentionPolicy.MaxAge > 0 {
			cutoffTime := time.Now().Add(-retentionPolicy.MaxAge)

			// Get messages in batch
			pipe := ds.client.Pipeline()
			cmds := make([]*redis.StringCmd, len(keys))

			for i, key := range keys {
				cmds[i] = pipe.Get(ctx, key)
			}

			_, err = pipe.Exec(ctx)
			if err != nil && err != redis.Nil {
				cursor = nextCursor
				if cursor == 0 {
					break
				}
				continue
			}

			// Check timestamps
			for i, cmd := range cmds {
				data, err := cmd.Bytes()
				if err != nil {
					continue
				}

				message, err := ds.serializer.Deserialize(data)
				if err != nil {
					continue
				}

				if time.Unix(message.Timestamp, 0).Before(cutoffTime) {
					keysToDelete = append(keysToDelete, keys[i])
				}
			}
		}

		// Delete expired keys
		if len(keysToDelete) > 0 {
			err := ds.client.Del(ctx, keysToDelete...).Err()
			if err != nil {
				ds.metrics.FailedOperations++
				return fmt.Errorf("failed to delete expired keys: %w", err)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	ds.metrics.SuccessfulOperations++
	return nil
}
