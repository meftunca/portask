package benchmarks

import (
	"context"
	"fmt"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// DragonflyKafkaStore implements kafka.MessageStore interface using Dragonfly (non-batch)
// This is a shared test helper used by multiple test files
type DragonflyKafkaStore struct {
	store *dragonfly.DragonflyStore
	ctx   context.Context
}

func (d *DragonflyKafkaStore) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	msg := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Partition: partition,
		Key:       string(key),
		Payload:   value,
		Timestamp: time.Now().UnixNano(),
		TTL:       int64(time.Hour),
	}

	err := d.store.Store(d.ctx, msg)
	if err != nil {
		return 0, err
	}

	return time.Now().UnixNano(), nil
}

func (d *DragonflyKafkaStore) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	return []*kafka.Message{}, nil
}

func (d *DragonflyKafkaStore) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{}, nil
}

func (d *DragonflyKafkaStore) CreateTopic(topic string, partitions int32, replication int16) error {
	return nil
}

func (d *DragonflyKafkaStore) DeleteTopic(topic string) error {
	return nil
}

