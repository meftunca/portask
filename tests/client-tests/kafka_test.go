package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// Test Kafka client connectivity to Portask
func main() {
	log.Printf("🔗 Testing Kafka client connection to Portask...")

	// Create a Kafka writer (producer)
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "test-topic",
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// Create a Kafka reader (consumer)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "test-topic",
		GroupID:  "test-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	log.Printf("✅ Kafka writer and reader created")

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

	log.Printf("📥 Starting to consume messages...")

	// Consume messages with timeout
	timeout := time.After(10 * time.Second)
	messageCount := 0

	go func() {
		for {
			select {
			case <-timeout:
				return
			default:
				// Set read deadline
				reader.SetOffset(0) // Start from beginning
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

				msg, err := reader.ReadMessage(ctx)
				cancel()

				if err != nil {
					if err == context.DeadlineExceeded {
						log.Printf("📋 No more messages available")
						return
					}
					log.Printf("❌ Error reading message: %v", err)
					return
				}

				messageCount++
				log.Printf("📨 Received: %s", string(msg.Value))

				if messageCount >= 5 {
					log.Printf("✅ Successfully received all %d messages", messageCount)
					return
				}
			}
		}
	}()

	// Wait for consumption to complete
	time.Sleep(12 * time.Second)
	log.Printf("✅ Kafka client test completed. Received %d messages", messageCount)
}
