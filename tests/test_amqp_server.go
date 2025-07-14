package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/meftunca/portask/pkg/amqp"
)

// Simple AMQP server test
type SimpleAMQPStore struct{}

func (s *SimpleAMQPStore) DeclareExchange(name, exchangeType string, durable, autoDelete bool) error {
	log.Printf("📦 Exchange declared: %s (type: %s)", name, exchangeType)
	return nil
}

func (s *SimpleAMQPStore) DeleteExchange(name string) error {
	log.Printf("🗑️  Exchange deleted: %s", name)
	return nil
}

func (s *SimpleAMQPStore) DeclareQueue(name string, durable, autoDelete, exclusive bool) error {
	log.Printf("📋 Queue declared: %s", name)
	return nil
}

func (s *SimpleAMQPStore) DeleteQueue(name string) error {
	log.Printf("🗑️  Queue deleted: %s", name)
	return nil
}

func (s *SimpleAMQPStore) BindQueue(queueName, exchangeName, routingKey string) error {
	log.Printf("🔗 Queue bound: %s -> %s (key: %s)", queueName, exchangeName, routingKey)
	return nil
}

func (s *SimpleAMQPStore) PublishMessage(exchange, routingKey string, body []byte) error {
	log.Printf("📤 Message published: %s/%s (%d bytes)", exchange, routingKey, len(body))
	return nil
}

func (s *SimpleAMQPStore) ConsumeMessages(queueName string, autoAck bool) ([][]byte, error) {
	log.Printf("📥 Messages consumed from: %s", queueName)
	return [][]byte{}, nil
}

// Additional methods to implement MessageStore interface
func (s *SimpleAMQPStore) StoreMessage(topic string, message []byte) error {
	return nil
}

func (s *SimpleAMQPStore) GetMessages(topic string, offset int64) ([][]byte, error) {
	return [][]byte{}, nil
}

func (s *SimpleAMQPStore) GetTopics() []string {
	return []string{}
}

func runSimpleTest() {
	log.Printf("🚀 Starting simple AMQP test server...")

	// Create simple store
	store := &SimpleAMQPStore{}

	// Create AMQP server
	server := amqp.NewEnhancedAMQPServer(":5672", store)

	// Start server
	if err := server.Start(); err != nil {
		log.Fatalf("❌ Failed to start AMQP server: %v", err)
	}

	log.Printf("✅ AMQP server started on port 5672")
	log.Printf("🧪 Test with: go run test_amqp_integration.go")

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Printf("🛑 Shutting down server...")
	if err := server.Stop(); err != nil {
		log.Printf("❌ Error stopping server: %v", err)
	}

	log.Printf("✅ Server stopped gracefully")
}
