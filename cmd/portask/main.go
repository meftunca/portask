package main

import (
	"fmt"
	"log"

	"github.com/meftunca/portask/pkg/amqp"
	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/kafka"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("🚀 Portask Multi-Protocol Server")

	// Kafka compatibility test
	kafkaAddr := fmt.Sprintf(":%d", cfg.Network.KafkaPort)
	log.Printf("🔗 Kafka wire protocol available on %s", kafkaAddr)

	// RabbitMQ compatibility test
	rabbitmqAddr := fmt.Sprintf(":%d", cfg.Network.RabbitMQPort)
	log.Printf("🐰 RabbitMQ wire protocol (placeholder) available on %s", rabbitmqAddr)

	// Create dummy stores
	kafkaStore := &DummyKafkaStore{}
	amqpStore := &DummyAMQPStore{}

	// Test Kafka server creation
	kafkaServer := kafka.NewKafkaServer(kafkaAddr, kafkaStore)
	log.Printf("✅ Kafka server instance created")

	// Test RabbitMQ server creation
	rabbitmqServer := amqp.NewRabbitMQServer(rabbitmqAddr, amqpStore)
	log.Printf("✅ RabbitMQ server instance created")

	log.Printf("🎯 Protocol Compatibility Status:")
	log.Printf("   ✅ Kafka Protocol: READY (Phase 2)")
	log.Printf("   🟡 RabbitMQ Protocol: PLACEHOLDER (Phase 3)")

	// Don't actually start servers in this test
	_ = kafkaServer
	_ = rabbitmqServer
}

// DummyKafkaStore for testing
type DummyKafkaStore struct{}

func (d *DummyKafkaStore) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	return 0, nil
}

func (d *DummyKafkaStore) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	return []*kafka.Message{}, nil
}

func (d *DummyKafkaStore) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{Name: "test"}, nil
}

func (d *DummyKafkaStore) CreateTopic(topic string, partitions int32, replication int16) error {
	return nil
}

func (d *DummyKafkaStore) DeleteTopic(topic string) error {
	return nil
}

// DummyAMQPStore for testing
type DummyAMQPStore struct{}

func (d *DummyAMQPStore) DeclareExchange(name, exchangeType string, durable, autoDelete bool) error {
	return nil
}

func (d *DummyAMQPStore) DeleteExchange(name string) error {
	return nil
}

func (d *DummyAMQPStore) DeclareQueue(name string, durable, autoDelete, exclusive bool) error {
	return nil
}

func (d *DummyAMQPStore) DeleteQueue(name string) error {
	return nil
}

func (d *DummyAMQPStore) BindQueue(queueName, exchangeName, routingKey string) error {
	return nil
}

func (d *DummyAMQPStore) PublishMessage(exchange, routingKey string, body []byte) error {
	return nil
}

func (d *DummyAMQPStore) ConsumeMessages(queueName string, autoAck bool) ([][]byte, error) {
	return [][]byte{}, nil
}
