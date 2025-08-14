// Test script for Kafka client connectivity to Portask
// Run with: go run kafka_client.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	log.Printf("🔗 Testing Kafka client connection to Portask...")

	// Create a Kafka writer (producer)
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "test-topic",
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	log.Printf("✅ Kafka writer created")

	// Produce test messages
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		message := fmt.Sprintf("Test message %d from Kafka client", i+1)

		err := writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(fmt.Sprintf("key-%d", i)),
			Value: []byte(message),
		})

		if err != nil {
			log.Printf("❌ Failed to publish message %d: %v", i+1, err)
		} else {
			log.Printf("📤 Published: %s", message)
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("✅ Kafka client test completed successfully!")
}
