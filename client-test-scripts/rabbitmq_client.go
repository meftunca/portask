// Test script for RabbitMQ client connectivity to Portask
// Run with: go run rabbitmq_client.go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/streadway/amqp"
)

func main() {
	log.Printf("🐰 Testing RabbitMQ client connection to Portask...")

	// Connect to Portask AMQP server
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatalf("❌ Failed to connect to Portask AMQP: %v", err)
	}
	defer conn.Close()
	log.Printf("✅ Connected to Portask AMQP server")

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("❌ Failed to open channel: %v", err)
	}
	defer ch.Close()
	log.Printf("✅ Channel opened successfully")

	// Declare exchange
	err = ch.ExchangeDeclare(
		"test_exchange", // name
		"direct",        // type
		true,            // durable
		false,           // auto-deleted
		false,           // internal
		false,           // no-wait
		nil,             // arguments
	)
	if err != nil {
		log.Fatalf("❌ Failed to declare exchange: %v", err)
	}
	log.Printf("✅ Exchange declared successfully")

	// Declare queue
	q, err := ch.QueueDeclare(
		"test_queue", // name
		false,        // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatalf("❌ Failed to declare queue: %v", err)
	}
	log.Printf("✅ Queue declared: %s", q.Name)

	// Bind queue to exchange
	err = ch.QueueBind(
		q.Name,          // queue name
		"test_key",      // routing key
		"test_exchange", // exchange
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("❌ Failed to bind queue: %v", err)
	}
	log.Printf("✅ Queue bound to exchange")

	// Publish test messages
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf("Test message %d from RabbitMQ client", i+1)
		err = ch.Publish(
			"test_exchange", // exchange
			"test_key",      // routing key
			false,           // mandatory
			false,           // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(body),
				Timestamp:   time.Now(),
			})
		if err != nil {
			log.Printf("❌ Failed to publish message %d: %v", i+1, err)
		} else {
			log.Printf("📤 Published: %s", body)
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("✅ RabbitMQ client test completed successfully!")
}
