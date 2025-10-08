package benchmarks

import (
	"context"
	"fmt"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// InMemoryKafkaAdapter adapts InMemoryStore to Kafka's MessageStore interface
type InMemoryKafkaAdapter struct {
	ctx   context.Context
	store *storage.InMemoryStore
}

// NewInMemoryKafkaAdapter creates a new in-memory Kafka adapter
func NewInMemoryKafkaAdapter(ctx context.Context) *InMemoryKafkaAdapter {
	return &InMemoryKafkaAdapter{
		ctx:   ctx,
		store: storage.NewInMemoryStore(),
	}
}

func (m *InMemoryKafkaAdapter) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	msg := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Partition: partition,
		Key:       string(key),
		Payload:   value,
		Timestamp: time.Now().UnixNano(),
		TTL:       int64(time.Hour),
	}

	if err := m.store.Store(m.ctx, msg); err != nil {
		return 0, err
	}

	return msg.Timestamp, nil
}

func (m *InMemoryKafkaAdapter) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	return []*kafka.Message{}, nil
}

func (m *InMemoryKafkaAdapter) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{}, nil
}

func (m *InMemoryKafkaAdapter) CreateTopic(topic string, partitions int32, replication int16) error {
	return nil
}

func (m *InMemoryKafkaAdapter) DeleteTopic(topic string) error {
	return nil
}

func (m *InMemoryKafkaAdapter) GetMetrics() map[string]int64 {
	return m.store.GetMetrics()
}

func (m *InMemoryKafkaAdapter) Reset() {
	m.store.Reset()
}

// InMemoryStorageAdapter adapts InMemoryStore to processor.StorageBackend interface
type InMemoryStorageAdapter struct {
	store *storage.InMemoryStore
}

// NewInMemoryStorageAdapter creates a new in-memory storage adapter
func NewInMemoryStorageAdapter() *InMemoryStorageAdapter {
	return &InMemoryStorageAdapter{
		store: storage.NewInMemoryStore(),
	}
}

// StoreBatch implements processor.StorageBackend interface
func (m *InMemoryStorageAdapter) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	return m.store.StoreBatch(ctx, batch)
}

func (m *InMemoryStorageAdapter) GetMetrics() map[string]int64 {
	return m.store.GetMetrics()
}

func (m *InMemoryStorageAdapter) Reset() {
	m.store.Reset()
}

