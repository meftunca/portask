package rocksdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/tecbot/gorocksdb"

	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// RocksDBStore implements MessageStore using RocksDB for local storage
type RocksDBStore struct {
	db       *gorocksdb.DB
	wo       *gorocksdb.WriteOptions
	ro       *gorocksdb.ReadOptions
	dataDir  string
	mu       sync.RWMutex
	
	// Metrics
	messagesWritten atomic.Int64
	messagesRead    atomic.Int64
	bytesWritten    atomic.Int64
	bytesRead       atomic.Int64
}

// Config for RocksDB
type Config struct {
	DataDir string
}

// NewRocksDBStore creates a new RocksDB storage backend
func NewRocksDBStore(config *Config) (*RocksDBStore, error) {
	if config.DataDir == "" {
		config.DataDir = "./rocksdb_data"
	}
	
	// Create data directory
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}
	
	// RocksDB options optimized for performance
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetCompression(gorocksdb.NoCompression) // Fast writes, no compression
	opts.SetWriteBufferSize(64 * 1024 * 1024)    // 64MB write buffer
	opts.SetMaxWriteBufferNumber(3)
	opts.SetTargetFileSizeBase(64 * 1024 * 1024)
	opts.SetMaxBackgroundCompactions(4)
	opts.SetMaxBackgroundFlushes(2)
	opts.SetBytesPerSync(1024 * 1024) // 1MB
	
	// Open database
	db, err := gorocksdb.OpenDb(opts, config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open rocksdb: %w", err)
	}
	
	// Write/Read options
	wo := gorocksdb.NewDefaultWriteOptions()
	wo.SetSync(false) // Async writes for speed
	
	ro := gorocksdb.NewDefaultReadOptions()
	
	return &RocksDBStore{
		db:      db,
		wo:      wo,
		ro:      ro,
		dataDir: config.DataDir,
	}, nil
}

// Store stores a single message
func (r *RocksDBStore) Store(ctx context.Context, message *types.PortaskMessage) error {
	// Serialize message
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}
	
	// Generate key
	key := []byte(fmt.Sprintf("msg:%s", message.ID))
	
	// Write to RocksDB
	if err := r.db.Put(r.wo, key, data); err != nil {
		return fmt.Errorf("rocksdb write failed: %w", err)
	}
	
	// Update metrics
	r.messagesWritten.Add(1)
	r.bytesWritten.Add(int64(len(data)))
	
	return nil
}

// StoreBatch stores multiple messages in a batch (optimized)
func (r *RocksDBStore) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	if batch == nil || len(batch.Messages) == 0 {
		return nil
	}
	
	// Use RocksDB WriteBatch for atomic batch writes
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	
	for _, message := range batch.Messages {
		// Serialize
		data, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("serialization failed for %s: %w", message.ID, err)
		}
		
		// Add to batch
		key := []byte(fmt.Sprintf("msg:%s", message.ID))
		wb.Put(key, data)
		
		// Update metrics
		r.bytesWritten.Add(int64(len(data)))
	}
	
	// Write entire batch atomically
	if err := r.db.Write(r.wo, wb); err != nil {
		return fmt.Errorf("batch write failed: %w", err)
	}
	
	r.messagesWritten.Add(int64(len(batch.Messages)))
	
	return nil
}

// Fetch retrieves messages (simplified for benchmarking)
func (r *RocksDBStore) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	// For benchmarking, we'll just return empty
	// Real implementation would use prefix scan
	return []*types.PortaskMessage{}, nil
}

// Close closes the RocksDB database
func (r *RocksDBStore) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.db != nil {
		r.db.Close()
		r.db = nil
	}
	
	return nil
}

// Stats returns storage statistics
func (r *RocksDBStore) Stats(ctx context.Context) (*storage.StorageStats, error) {
	return &storage.StorageStats{
		MessageCount:     r.messagesWritten.Load(),
		StorageUsedBytes: r.bytesWritten.Load(),
	}, nil
}

// GetMetrics returns performance metrics
func (r *RocksDBStore) GetMetrics() map[string]int64 {
	return map[string]int64{
		"messages_written": r.messagesWritten.Load(),
		"messages_read":    r.messagesRead.Load(),
		"bytes_written":    r.bytesWritten.Load(),
		"bytes_read":       r.bytesRead.Load(),
	}
}

// CleanupDataDir removes the RocksDB data directory
func (r *RocksDBStore) CleanupDataDir() error {
	if err := r.Close(); err != nil {
		return err
	}
	return os.RemoveAll(r.dataDir)
}

// Stub implementations for MessageStore interface
func (r *RocksDBStore) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	return nil, nil
}

func (r *RocksDBStore) Delete(ctx context.Context, messageID types.MessageID) error {
	return nil
}

func (r *RocksDBStore) DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error {
	return nil
}

func (r *RocksDBStore) CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error {
	return nil
}

func (r *RocksDBStore) DeleteTopic(ctx context.Context, topic types.TopicName) error {
	return nil
}

func (r *RocksDBStore) GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error) {
	return nil, nil
}

func (r *RocksDBStore) ListTopics(ctx context.Context) ([]*types.TopicInfo, error) {
	return nil, nil
}

func (r *RocksDBStore) TopicExists(ctx context.Context, topic types.TopicName) (bool, error) {
	return false, nil
}

func (r *RocksDBStore) GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error) {
	return nil, nil
}

func (r *RocksDBStore) GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error) {
	return 0, nil
}

func (r *RocksDBStore) GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (r *RocksDBStore) GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (r *RocksDBStore) CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error {
	return nil
}

func (r *RocksDBStore) CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error {
	return nil
}

func (r *RocksDBStore) GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error) {
	return nil, nil
}

func (r *RocksDBStore) GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error) {
	return nil, nil
}

func (r *RocksDBStore) ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error) {
	return nil, nil
}

func (r *RocksDBStore) Ping(ctx context.Context) error {
	return nil
}

func (r *RocksDBStore) Cleanup(ctx context.Context, retentionPolicy *storage.RetentionPolicy) error {
	return nil
}

