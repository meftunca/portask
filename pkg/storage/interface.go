package storage

import (
	"context"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

// MessageStore defines the interface for message storage operations
type MessageStore interface {
	// Message operations
	Store(ctx context.Context, message *types.PortaskMessage) error
	StoreBatch(ctx context.Context, batch *types.MessageBatch) error
	Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error)
	FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error)
	Delete(ctx context.Context, messageID types.MessageID) error
	DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error

	// Topic operations
	CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error
	DeleteTopic(ctx context.Context, topic types.TopicName) error
	GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error)
	ListTopics(ctx context.Context) ([]*types.TopicInfo, error)
	TopicExists(ctx context.Context, topic types.TopicName) (bool, error)

	// Partition operations
	GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error)
	GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error)
	GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error)
	GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error)

	// Consumer operations
	CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error
	CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error
	GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error)
	GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error)
	ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error)

	// Health and maintenance
	Ping(ctx context.Context) error
	Stats(ctx context.Context) (*StorageStats, error)
	Cleanup(ctx context.Context, retentionPolicy *RetentionPolicy) error
	Close() error
}

// StorageStats represents storage statistics
type StorageStats struct {
	// Connection stats
	ActiveConnections int   `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`
	ConnectionErrors  int64 `json:"connection_errors"`

	// Operation stats
	MessageCount         int64 `json:"message_count"`
	TopicCount           int64 `json:"topic_count"`
	ConsumerCount        int64 `json:"consumer_count"`
	TotalOperations      int64 `json:"total_operations"`
	SuccessfulOperations int64 `json:"successful_operations"`
	FailedOperations     int64 `json:"failed_operations"`

	// Performance stats
	AvgLatencyMs     float64       `json:"avg_latency_ms"`
	P99LatencyMs     float64       `json:"p99_latency_ms"`
	ThroughputPerSec float64       `json:"throughput_per_sec"`
	AvgResponseTime  time.Duration `json:"avg_response_time"`
	P95ResponseTime  time.Duration `json:"p95_response_time"`
	P99ResponseTime  time.Duration `json:"p99_response_time"`

	// Storage stats
	StorageUsedBytes    int64   `json:"storage_used_bytes"`
	StorageTotalBytes   int64   `json:"storage_total_bytes"`
	StorageUsagePercent float64 `json:"storage_usage_percent"`
	UsedMemory          int64   `json:"used_memory"`
	TotalMemory         int64   `json:"total_memory"`
	KeyCount            int64   `json:"key_count"`
	ExpiringKeys        int64   `json:"expiring_keys"`

	// Error stats
	TotalErrors   int64 `json:"total_errors"`
	TimeoutErrors int64 `json:"timeout_errors"`
	NetworkErrors int64 `json:"network_errors"`

	// Health
	Status          string        `json:"status"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	Uptime          time.Duration `json:"uptime"`
}

// RetentionPolicy defines data retention rules
type RetentionPolicy struct {
	// Time-based retention
	MaxAge time.Duration `json:"max_age"`

	// Size-based retention
	MaxSizeBytes int64 `json:"max_size_bytes"`
	MaxMessages  int64 `json:"max_messages"`

	// Cleanup strategy
	CleanupStrategy CleanupStrategy `json:"cleanup_strategy"`

	// Cleanup batch size
	BatchSize int `json:"batch_size"`
}

// CleanupStrategy defines how to clean up old data
type CleanupStrategy string

const (
	CleanupOldest     CleanupStrategy = "oldest"   // Remove oldest messages first
	CleanupLRU        CleanupStrategy = "lru"      // Remove least recently used
	CleanupByPriority CleanupStrategy = "priority" // Remove low priority first
	CleanupByTopic    CleanupStrategy = "topic"    // Remove by topic pattern
)

// StorageConfig holds storage-specific configuration
type StorageConfig struct {
	// Connection settings
	MaxConnections     int           `json:"max_connections"`
	MaxIdleConnections int           `json:"max_idle_connections"`
	ConnectionTimeout  time.Duration `json:"connection_timeout"`
	ReadTimeout        time.Duration `json:"read_timeout"`
	WriteTimeout       time.Duration `json:"write_timeout"`
	IdleTimeout        time.Duration `json:"idle_timeout"`

	// Retry settings
	MaxRetries   int           `json:"max_retries"`
	RetryDelay   time.Duration `json:"retry_delay"`
	RetryBackoff float64       `json:"retry_backoff"`

	// Performance settings
	BatchSize         int           `json:"batch_size"`
	EnableBatching    bool          `json:"enable_batching"`
	BatchTimeout      time.Duration `json:"batch_timeout"`
	EnableCompression bool          `json:"enable_compression"`
	EnablePipelining  bool          `json:"enable_pipelining"`

	// Health check settings
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`

	// Retention settings
	RetentionPolicy *RetentionPolicy `json:"retention_policy"`
}

// StorageFactory creates storage instances
type StorageFactory interface {
	// CreateStorage creates a new storage instance
	CreateStorage(config *StorageConfig) (MessageStore, error)

	// GetSupportedTypes returns supported storage types
	GetSupportedTypes() []string

	// ValidateConfig validates storage configuration
	ValidateConfig(config *StorageConfig) error
}

// StorageManager manages multiple storage backends
type StorageManager struct {
	stores  map[string]MessageStore
	primary MessageStore
	config  *StorageConfig
}

// NewStorageManager creates a new storage manager
func NewStorageManager(config *StorageConfig) *StorageManager {
	return &StorageManager{
		stores: make(map[string]MessageStore),
		config: config,
	}
}

// SetPrimary sets the primary storage backend
func (sm *StorageManager) SetPrimary(store MessageStore) {
	sm.primary = store
}

// GetPrimary returns the primary storage backend
func (sm *StorageManager) GetPrimary() MessageStore {
	return sm.primary
}

// AddStore adds a named storage backend
func (sm *StorageManager) AddStore(name string, store MessageStore) {
	sm.stores[name] = store
}

// GetStore returns a named storage backend
func (sm *StorageManager) GetStore(name string) (MessageStore, bool) {
	store, exists := sm.stores[name]
	return store, exists
}

// CloseAll closes all storage backends
func (sm *StorageManager) CloseAll() error {
	var errors []error

	for name, store := range sm.stores {
		if err := store.Close(); err != nil {
			errors = append(errors, types.NewPortaskError(types.ErrCodeStorageError, "failed to close storage").
				WithDetail("store", name).
				WithDetail("error", err.Error()))
		}
	}

	if len(errors) > 0 {
		collector := types.NewErrorCollector()
		for _, err := range errors {
			collector.Add(err)
		}
		return collector.ToError()
	}

	return nil
}

// HealthCheck performs health check on all storage backends
func (sm *StorageManager) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)

	for name, store := range sm.stores {
		if err := store.Ping(ctx); err != nil {
			results[name] = err
		} else {
			results[name] = nil
		}
	}

	return results
}

// StorageMetrics collects metrics from storage operations
type StorageMetrics struct {
	// Operation counters
	StoreOps  int64
	FetchOps  int64
	DeleteOps int64
	BatchOps  int64

	// Error counters
	StoreErrors  int64
	FetchErrors  int64
	DeleteErrors int64
	BatchErrors  int64

	// Latency tracking
	StoreLatency  time.Duration
	FetchLatency  time.Duration
	DeleteLatency time.Duration
	BatchLatency  time.Duration

	// Throughput tracking
	MessagesPerSec float64
	BytesPerSec    float64

	// Connection metrics
	ActiveConns int
	IdleConns   int
	ConnErrors  int64

	// Last update
	LastUpdate time.Time
}

// MetricsCollector collects storage metrics
type MetricsCollector interface {
	// Record operation metrics
	RecordStoreOperation(duration time.Duration, err error)
	RecordFetchOperation(duration time.Duration, messageCount int, err error)
	RecordDeleteOperation(duration time.Duration, err error)
	RecordBatchOperation(duration time.Duration, messageCount int, err error)

	// Record connection metrics
	RecordConnectionEvent(event string)
	RecordConnectionError(err error)

	// Get current metrics
	GetMetrics() *StorageMetrics

	// Reset metrics
	Reset()
}

// Transaction represents a storage transaction
type Transaction interface {
	// Commit commits the transaction
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction
	Rollback(ctx context.Context) error

	// Store operations within transaction
	Store(ctx context.Context, message *types.PortaskMessage) error
	StoreBatch(ctx context.Context, batch *types.MessageBatch) error
	Delete(ctx context.Context, messageID types.MessageID) error

	// Offset operations within transaction
	CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error
}

// TransactionalStore extends MessageStore with transaction support
type TransactionalStore interface {
	MessageStore

	// BeginTransaction starts a new transaction
	BeginTransaction(ctx context.Context) (Transaction, error)
}

// QueryBuilder builds storage queries
type QueryBuilder interface {
	// Topic filtering
	Topic(topic types.TopicName) QueryBuilder
	Topics(topics []types.TopicName) QueryBuilder

	// Partition filtering
	Partition(partition int32) QueryBuilder
	Partitions(partitions []int32) QueryBuilder

	// Offset range
	OffsetRange(start, end int64) QueryBuilder
	OffsetFrom(start int64) QueryBuilder

	// Time range
	TimeRange(start, end time.Time) QueryBuilder
	Since(duration time.Duration) QueryBuilder

	// Message filtering
	MessageID(id types.MessageID) QueryBuilder
	MessageIDs(ids []types.MessageID) QueryBuilder
	Priority(priority types.MessagePriority) QueryBuilder
	Status(status types.MessageStatus) QueryBuilder

	// Consumer filtering
	Consumer(consumerID types.ConsumerID) QueryBuilder

	// Sorting and limiting
	OrderBy(field string, ascending bool) QueryBuilder
	Limit(limit int) QueryBuilder
	Offset(offset int) QueryBuilder

	// Execute query
	Execute(ctx context.Context, store MessageStore) ([]*types.PortaskMessage, error)
	Count(ctx context.Context, store MessageStore) (int64, error)
}

// IndexManager manages storage indexes for performance
type IndexManager interface {
	// Create indexes
	CreateTopicIndex(ctx context.Context, topic types.TopicName) error
	CreateTimeIndex(ctx context.Context, topic types.TopicName) error
	CreateConsumerIndex(ctx context.Context, consumerID types.ConsumerID) error

	// Drop indexes
	DropTopicIndex(ctx context.Context, topic types.TopicName) error
	DropTimeIndex(ctx context.Context, topic types.TopicName) error
	DropConsumerIndex(ctx context.Context, consumerID types.ConsumerID) error

	// Index stats
	GetIndexStats(ctx context.Context) (map[string]interface{}, error)

	// Optimize indexes
	OptimizeIndexes(ctx context.Context) error
}

// CacheStore provides caching layer for storage
type CacheStore interface {
	// Cache operations
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
	Clear() error

	// Cache stats
	Stats() map[string]interface{}

	// Size management
	Size() int64
	MaxSize() int64
	SetMaxSize(maxSize int64)
}

// CompositStore combines multiple storage backends
type CompositeStore struct {
	primary   MessageStore
	secondary []MessageStore
	config    *StorageConfig
}

// NewCompositeStore creates a new composite store
func NewCompositeStore(primary MessageStore, secondary []MessageStore, config *StorageConfig) *CompositeStore {
	return &CompositeStore{
		primary:   primary,
		secondary: secondary,
		config:    config,
	}
}

// Store implements MessageStore interface with replication
func (cs *CompositeStore) Store(ctx context.Context, message *types.PortaskMessage) error {
	// Store to primary first
	if err := cs.primary.Store(ctx, message); err != nil {
		return err
	}

	// Replicate to secondary stores (best effort)
	for _, store := range cs.secondary {
		go func(s MessageStore) {
			if err := s.Store(context.Background(), message); err != nil {
				// Log error but don't fail the operation
				// TODO: Add proper logging
			}
		}(store)
	}

	return nil
}

// Fetch implements MessageStore interface with fallback
func (cs *CompositeStore) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	// Try primary first
	messages, err := cs.primary.Fetch(ctx, topic, partition, offset, limit)
	if err == nil {
		return messages, nil
	}

	// Try secondary stores as fallback
	for _, store := range cs.secondary {
		if messages, err := store.Fetch(ctx, topic, partition, offset, limit); err == nil {
			return messages, nil
		}
	}

	return nil, err
}

// Close implements MessageStore interface
func (cs *CompositeStore) Close() error {
	var errors []error

	if err := cs.primary.Close(); err != nil {
		errors = append(errors, err)
	}

	for _, store := range cs.secondary {
		if err := store.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		collector := types.NewErrorCollector()
		for _, err := range errors {
			collector.Add(err)
		}
		return collector.ToError()
	}

	return nil
}

// Implement other MessageStore methods with similar primary/secondary pattern...
// (For brevity, not implementing all methods here, but they would follow the same pattern)
