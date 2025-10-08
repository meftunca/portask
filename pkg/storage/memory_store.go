package storage

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/meftunca/portask/pkg/types"
)

// InMemoryStore is a fast in-memory storage for bypass testing
// This helps identify if storage is the bottleneck
type InMemoryStore struct {
	messages  sync.Map // topic -> []messages
	counter   atomic.Int64
	writeOps  atomic.Int64
	readOps   atomic.Int64
	bytesIn   atomic.Int64
	bytesOut  atomic.Int64
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{}
}

// Store stores a message in memory
func (s *InMemoryStore) Store(ctx context.Context, msg *types.PortaskMessage) error {
	s.counter.Add(1)
	s.writeOps.Add(1)
	s.bytesIn.Add(int64(len(msg.Payload)))
	
	// Don't actually store to avoid memory bloat during benchmarks
	// Just count the operation
	return nil
}

// StoreBatch stores multiple messages in a batch
func (s *InMemoryStore) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	if batch == nil || len(batch.Messages) == 0 {
		return nil
	}
	
	s.counter.Add(int64(len(batch.Messages)))
	s.writeOps.Add(1) // One batch write
	
	// Count bytes
	for _, msg := range batch.Messages {
		s.bytesIn.Add(int64(len(msg.Payload)))
	}
	
	// Don't actually store to avoid memory bloat
	return nil
}

// Fetch retrieves messages (not implemented for bypass test)
func (s *InMemoryStore) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	s.readOps.Add(1)
	return []*types.PortaskMessage{}, nil
}

// Delete deletes messages (not implemented for bypass test)
func (s *InMemoryStore) Delete(ctx context.Context, topic types.TopicName, messageIDs []types.MessageID) error {
	return nil
}

// Stats returns storage statistics
func (s *InMemoryStore) Stats(ctx context.Context) (*StorageStats, error) {
	return &StorageStats{
		MessageCount:     s.counter.Load(),
		StorageUsedBytes: s.bytesIn.Load(),
		TopicCount:       0,
	}, nil
}

// Connect is a no-op for in-memory store
func (s *InMemoryStore) Connect(ctx context.Context) error {
	return nil
}

// Close is a no-op for in-memory store
func (s *InMemoryStore) Close() error {
	return nil
}

// GetMetrics returns performance metrics
func (s *InMemoryStore) GetMetrics() map[string]int64 {
	return map[string]int64{
		"messages":   s.counter.Load(),
		"write_ops":  s.writeOps.Load(),
		"read_ops":   s.readOps.Load(),
		"bytes_in":   s.bytesIn.Load(),
		"bytes_out":  s.bytesOut.Load(),
	}
}

// Reset resets all counters
func (s *InMemoryStore) Reset() {
	s.counter.Store(0)
	s.writeOps.Store(0)
	s.readOps.Store(0)
	s.bytesIn.Store(0)
	s.bytesOut.Store(0)
	s.messages = sync.Map{}
}

// Stub implementations for MessageStore interface

func (s *InMemoryStore) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	return nil, nil
}

func (s *InMemoryStore) DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error {
	return nil
}

func (s *InMemoryStore) CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error {
	return nil
}

func (s *InMemoryStore) DeleteTopic(ctx context.Context, topic types.TopicName) error {
	return nil
}

func (s *InMemoryStore) GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error) {
	return nil, nil
}

func (s *InMemoryStore) ListTopics(ctx context.Context) ([]*types.TopicInfo, error) {
	return nil, nil
}

func (s *InMemoryStore) TopicExists(ctx context.Context, topic types.TopicName) (bool, error) {
	return false, nil
}

func (s *InMemoryStore) GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error) {
	return nil, nil
}

func (s *InMemoryStore) GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error) {
	return 0, nil
}

func (s *InMemoryStore) GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (s *InMemoryStore) GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}

func (s *InMemoryStore) CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error {
	return nil
}

func (s *InMemoryStore) CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error {
	return nil
}

func (s *InMemoryStore) GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error) {
	return nil, nil
}

func (s *InMemoryStore) GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error) {
	return nil, nil
}

func (s *InMemoryStore) ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error) {
	return nil, nil
}

func (s *InMemoryStore) Ping(ctx context.Context) error {
	return nil
}

func (s *InMemoryStore) Cleanup(ctx context.Context, retentionPolicy *RetentionPolicy) error {
	return nil
}

