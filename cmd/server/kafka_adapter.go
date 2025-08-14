package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/queue"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

type KafkaStorageAdapter struct {
	storage    storage.MessageStore
	messageBus *queue.MessageBus
}

func (k *KafkaStorageAdapter) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	message := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("kafka-%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Timestamp: time.Now().Unix(),
		Payload:   value,
	}

	ctx := context.Background()
	if err := k.storage.Store(ctx, message); err != nil {
		return 0, err
	}

	if err := k.messageBus.Publish(message); err != nil {
		log.Printf("⚠️  Failed to publish to message bus: %v", err)
	}

	return time.Now().UnixNano(), nil
}

func (k *KafkaStorageAdapter) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	ctx := context.Background()
	messages, err := k.storage.Fetch(ctx, types.TopicName(topic), partition, offset, 100)
	if err != nil {
		return nil, err
	}

	kafkaMessages := make([]*kafka.Message, len(messages))
	for i, msg := range messages {
		kafkaMessages[i] = &kafka.Message{
			Offset:    int64(i) + offset,
			Key:       []byte(msg.ID),
			Value:     msg.Payload,
			Timestamp: time.Unix(msg.Timestamp, 0),
		}
	}

	return kafkaMessages, nil
}

func (k *KafkaStorageAdapter) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	if len(topics) == 0 {
		return &kafka.TopicMetadata{
			Name:       "default-topic",
			Partitions: []kafka.PartitionMetadata{{ID: 0, Leader: 0}},
		}, nil
	}

	return &kafka.TopicMetadata{
		Name:       topics[0],
		Partitions: []kafka.PartitionMetadata{{ID: 0, Leader: 0}},
	}, nil
}

func (k *KafkaStorageAdapter) CreateTopic(topic string, partitions int32, replication int16) error {
	ctx := context.Background()
	topicInfo := &types.TopicInfo{
		Name:              types.TopicName(topic),
		Partitions:        partitions,
		ReplicationFactor: replication,
		CreatedAt:         time.Now().Unix(),
	}

	log.Printf("📝 Kafka CreateTopic: %s (partitions: %d)", topic, partitions)
	return k.storage.CreateTopic(ctx, topicInfo)
}

func (k *KafkaStorageAdapter) DeleteTopic(topic string) error {
	ctx := context.Background()
	log.Printf("🗑️ Kafka DeleteTopic: %s", topic)
	return k.storage.DeleteTopic(ctx, types.TopicName(topic))
}

// NewKafkaServer creates a new Kafka server using the real kafka package implementation
func NewKafkaServer(addr string, adapter *KafkaStorageAdapter) *kafka.KafkaServer {
	// Use the real Kafka server from pkg/kafka
	return kafka.NewKafkaServer(addr, adapter)
}
