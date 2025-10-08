package badgerdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// BadgerStore implements MessageStore using BadgerDB (pure Go!)
type BadgerStore struct {
	db      *badger.DB
	dataDir string

	// Metrics
	messagesWritten atomic.Int64
	messagesRead    atomic.Int64
	bytesWritten    atomic.Int64
}

// Config for BadgerDB
type Config struct {
	DataDir string
}

// NewBadgerStore creates a new BadgerDB storage backend
func NewBadgerStore(config *Config) (*BadgerStore, error) {
	if config.DataDir == "" {
		config.DataDir = "./badger_data"
	}

	// Create data directory
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	// BadgerDB options optimized for write throughput
	opts := badger.DefaultOptions(config.DataDir)
	opts.SyncWrites = false          // Async for speed
	opts.NumVersionsToKeep = 1       // Don't keep old versions
	opts.CompactL0OnClose = false    // Fast shutdown
	opts.ValueLogFileSize = 64 << 20 // 64MB
	opts.MemTableSize = 64 << 20     // 64MB
	opts.NumMemtables = 3            // Concurrent writes
	opts.NumLevelZeroTables = 4
	opts.NumLevelZeroTablesStall = 8
	opts.Logger = nil // Disable logging for speed

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger: %w", err)
	}

	return &BadgerStore{
		db:      db,
		dataDir: config.DataDir,
	}, nil
}

// Store stores a single message
func (b *BadgerStore) Store(ctx context.Context, message *types.PortaskMessage) error {
	// Serialize
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Write to BadgerDB
	key := []byte(fmt.Sprintf("msg:%s", message.ID))

	err = b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})

	if err != nil {
		return err
	}

	// Update metrics
	b.messagesWritten.Add(1)
	b.bytesWritten.Add(int64(len(data)))

	return nil
}

// StoreBatch stores multiple messages in a batch (optimized!)
func (b *BadgerStore) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	if batch == nil || len(batch.Messages) == 0 {
		return nil
	}

	// Use BadgerDB transaction for batch write
	err := b.db.Update(func(txn *badger.Txn) error {
		for _, message := range batch.Messages {
			// Serialize
			data, err := json.Marshal(message)
			if err != nil {
				return err
			}

			// Write to transaction
			key := []byte(fmt.Sprintf("msg:%s", message.ID))
			if err := txn.Set(key, data); err != nil {
				return err
			}

			b.bytesWritten.Add(int64(len(data)))
		}
		return nil
	})

	if err != nil {
		return err
	}

	b.messagesWritten.Add(int64(len(batch.Messages)))

	return nil
}

// Fetch retrieves messages (simplified)
func (b *BadgerStore) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	return []*types.PortaskMessage{}, nil
}

// Close closes the database
func (b *BadgerStore) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

// Stats returns storage statistics
func (b *BadgerStore) Stats(ctx context.Context) (*storage.StorageStats, error) {
	return &storage.StorageStats{
		MessageCount:     b.messagesWritten.Load(),
		StorageUsedBytes: b.bytesWritten.Load(),
	}, nil
}

// GetMetrics returns performance metrics
func (b *BadgerStore) GetMetrics() map[string]int64 {
	return map[string]int64{
		"messages_written": b.messagesWritten.Load(),
		"messages_read":    b.messagesRead.Load(),
		"bytes_written":    b.bytesWritten.Load(),
	}
}

// CleanupDataDir removes the BadgerDB data directory
func (b *BadgerStore) CleanupDataDir() error {
	if err := b.Close(); err != nil {
		return err
	}
	return os.RemoveAll(b.dataDir)
}

// Stub implementations for MessageStore interface
func (b *BadgerStore) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	return nil, nil
}

func (b *BadgerStore) Delete(ctx context.Context, messageID types.MessageID) error {
	return nil
}

func (b *BadgerStore) DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error {
	return nil
}

func (b *BadgerStore) CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error {
	return nil
}

func (b *BadgerStore) DeleteTopic(ctx context.Context, topic types.TopicName) error {
	return nil
}

func (b *BadgerStore) GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error) {
	return nil, nil
}

func (b *BadgerStore) ListTopics(ctx context.Context) ([]*types.TopicInfo, error) {
	return nil, nil
}

func (b *BadgerStore) TopicExists(ctx context.Context, topic types.TopicName) (bool, error) {
	return false, nil
}

func (b *BadgerStore) GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error) {
	return nil, nil
}

func (b *BadgerStore) GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error) {
	return 0, nil
}

func (b *BadgerStore) GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (b *BadgerStore) GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (b *BadgerStore) CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error {
	return nil
}

func (b *BadgerStore) CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error {
	return nil
}

func (b *BadgerStore) GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error) {
	return nil, nil
}

func (b *BadgerStore) GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error) {
	return nil, nil
}

func (b *BadgerStore) ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error) {
	return nil, nil
}

func (b *BadgerStore) Ping(ctx context.Context) error {
	return nil
}

func (b *BadgerStore) Cleanup(ctx context.Context, retentionPolicy *storage.RetentionPolicy) error {
	return nil
}
