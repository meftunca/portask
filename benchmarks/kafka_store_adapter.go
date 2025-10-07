package benchmarks

import (
	"context"
	"fmt"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// DragonflyKafkaStoreAdapter adapts DragonflyStore to Kafka's MessageStore interface
type DragonflyKafkaStoreAdapter struct {
	ctx   context.Context
	store *dragonfly.DragonflyStore
}

// NewDragonflyKafkaStore creates a new Dragonfly Kafka store adapter
func NewDragonflyKafkaStore(ctx context.Context, store *dragonfly.DragonflyStore) *DragonflyKafkaStoreAdapter {
	return &DragonflyKafkaStoreAdapter{ctx: ctx, store: store}
}

func (d *DragonflyKafkaStoreAdapter) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	msg := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Partition: partition,
		Key:       string(key),
		Payload:   value,
		Timestamp: time.Now().UnixNano(),
		TTL:       int64(time.Hour),
	}

	if err := d.store.Store(d.ctx, msg); err != nil {
		return 0, err
	}

	return msg.Timestamp, nil
}

func (d *DragonflyKafkaStoreAdapter) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	messages, err := d.store.Fetch(d.ctx, types.TopicName(topic), partition, offset, 100)
	if err != nil {
		return nil, err
	}

	kafkaMessages := make([]*kafka.Message, 0, len(messages))
	for _, msg := range messages {
		kafkaMessages = append(kafkaMessages, &kafka.Message{
			Offset: msg.Timestamp,
			Key:    []byte(msg.Key),
			Value:  msg.Payload,
		})
	}

	return kafkaMessages, nil
}

func (d *DragonflyKafkaStoreAdapter) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{}, nil
}

func (d *DragonflyKafkaStoreAdapter) CreateTopic(topic string, partitions int32, replication int16) error {
	return nil
}

func (d *DragonflyKafkaStoreAdapter) DeleteTopic(topic string) error {
	return nil
}
