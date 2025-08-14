package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/meftunca/portask/pkg/queue"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

type AMQPStorageAdapter struct {
	storage    storage.MessageStore
	messageBus *queue.MessageBus
}

func (a *AMQPStorageAdapter) DeclareExchange(name, exchangeType string, durable, autoDelete bool) error {
	log.Printf("📝 AMQP DeclareExchange: %s (type: %s)", name, exchangeType)
	return nil
}

func (a *AMQPStorageAdapter) DeleteExchange(name string) error {
	log.Printf("🗑️ AMQP DeleteExchange: %s", name)
	return nil
}

func (a *AMQPStorageAdapter) DeclareQueue(name string, durable, autoDelete, exclusive bool) error {
	log.Printf("📝 AMQP DeclareQueue: %s", name)
	ctx := context.Background()
	topicInfo := &types.TopicInfo{
		Name:              types.TopicName(name),
		Partitions:        1,
		ReplicationFactor: 1,
		CreatedAt:         time.Now().Unix(),
	}
	return a.storage.CreateTopic(ctx, topicInfo)
}

func (a *AMQPStorageAdapter) DeleteQueue(name string) error {
	log.Printf("🗑️ AMQP DeleteQueue: %s", name)
	ctx := context.Background()
	return a.storage.DeleteTopic(ctx, types.TopicName(name))
}

func (a *AMQPStorageAdapter) BindQueue(queueName, exchangeName, routingKey string) error {
	log.Printf("🔗 AMQP BindQueue: %s -> %s (%s)", queueName, exchangeName, routingKey)
	return nil
}

func (a *AMQPStorageAdapter) PublishMessage(exchange, routingKey string, body []byte) error {
	message := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("amqp-%d", time.Now().UnixNano())),
		Topic:     types.TopicName(fmt.Sprintf("%s.%s", exchange, routingKey)),
		Timestamp: time.Now().Unix(),
		Payload:   body,
	}

	ctx := context.Background()
	if err := a.storage.Store(ctx, message); err != nil {
		return err
	}

	log.Printf("📤 AMQP PublishMessage: %s/%s (%d bytes)", exchange, routingKey, len(body))
	return a.messageBus.Publish(message)
}

func (a *AMQPStorageAdapter) ConsumeMessages(queueName string, autoAck bool) ([][]byte, error) {
	ctx := context.Background()
	messages, err := a.storage.Fetch(ctx, types.TopicName(queueName), 0, 0, 10)
	if err != nil {
		return nil, err
	}

	result := make([][]byte, len(messages))
	for i, msg := range messages {
		result[i] = msg.Payload
	}

	log.Printf("📥 AMQP ConsumeMessages: %s (%d messages)", queueName, len(result))
	return result, nil
}

// Additional methods to implement MessageStore interface
func (a *AMQPStorageAdapter) StoreMessage(topic string, message []byte) error {
	ctx := context.Background()
	msg := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("amqp_%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Payload:   message,
		Timestamp: time.Now().Unix(),
	}
	return a.storage.Store(ctx, msg)
}

func (a *AMQPStorageAdapter) GetMessages(topic string, offset int64) ([][]byte, error) {
	ctx := context.Background()
	messages, err := a.storage.Fetch(ctx, types.TopicName(topic), 0, offset, 100)
	if err != nil {
		return nil, err
	}

	result := make([][]byte, len(messages))
	for i, msg := range messages {
		result[i] = msg.Payload
	}
	return result, nil
}

func (a *AMQPStorageAdapter) GetTopics() []string {
	return []string{}
}
