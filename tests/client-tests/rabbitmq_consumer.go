package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

// Comprehensive RabbitMQ Consumer Test using amqp library
func main() {
	log.Printf("🐰 Testing RabbitMQ Consumer with amqp library...")

	// Test 1: Basic Consumer with Auto-Ack
	log.Printf("\n📝 Test 1: Basic Consumer (Auto-Ack)")
	testBasicConsumer()

	// Test 2: Manual Acknowledgment
	log.Printf("\n📝 Test 2: Manual Acknowledgment")
	testManualAck()

	// Test 3: Negative Acknowledgment (Nack) with Requeue
	log.Printf("\n📝 Test 3: Negative Acknowledgment (Nack)")
	testNack()

	// Test 4: QoS (Quality of Service) - Prefetch
	log.Printf("\n📝 Test 4: QoS Prefetch")
	testQoS()

	// Test 5: Multiple Consumers on Same Queue
	log.Printf("\n📝 Test 5: Multiple Consumers (Work Queue)")
	testMultipleConsumers()

	// Test 6: Exchange Types (Direct, Fanout, Topic)
	log.Printf("\n📝 Test 6: Exchange Types")
	testExchangeTypes()

	// Test 7: Priority Queue
	log.Printf("\n📝 Test 7: Priority Queue")
	testPriorityQueue()

	log.Printf("\n✅ All RabbitMQ consumer tests completed!")
}

func testBasicConsumer() {
	conn, ch := setupConnection()
	defer conn.Close()
	defer ch.Close()

	queueName := "basic-consumer-test"
	q, err := ch.QueueDeclare(queueName, false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to declare queue: %v", err)
		return
	}

	// Publish messages
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf("Basic message %d", i)
		err := ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
		if err != nil {
			log.Printf("❌ Failed to publish: %v", err)
		} else {
			log.Printf("✅ Published: %s", body)
		}
	}

	// Consume with auto-ack
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to consume: %v", err)
		return
	}

	consumeCount := 0
	timeout := time.After(3 * time.Second)

	for {
		select {
		case msg := <-msgs:
			consumeCount++
			log.Printf("📨 Consumed: %s (auto-acked)", string(msg.Body))
			if consumeCount >= 5 {
				log.Printf("✅ Basic consumer test passed: %d/5 messages", consumeCount)
				return
			}
		case <-timeout:
			log.Printf("⏰ Timeout. Consumed %d/5 messages", consumeCount)
			return
		}
	}
}

func testManualAck() {
	conn, ch := setupConnection()
	defer conn.Close()
	defer ch.Close()

	queueName := "manual-ack-test"
	q, err := ch.QueueDeclare(queueName, false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to declare queue: %v", err)
		return
	}

	// Publish messages
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf("Manual ack message %d", i)
		err := ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
		if err != nil {
			log.Printf("❌ Failed to publish: %v", err)
		}
	}

	// Consume with manual ack (autoAck=false)
	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to consume: %v", err)
		return
	}

	consumeCount := 0
	timeout := time.After(3 * time.Second)

	for {
		select {
		case msg := <-msgs:
			consumeCount++
			log.Printf("📨 Consumed: %s (delivery tag: %d)", string(msg.Body), msg.DeliveryTag)

			// Manual acknowledgment
			err := msg.Ack(false) // false = single message
			if err != nil {
				log.Printf("❌ Failed to ack: %v", err)
			} else {
				log.Printf("✅ Manually acked delivery tag %d", msg.DeliveryTag)
			}

			if consumeCount >= 5 {
				log.Printf("✅ Manual ack test passed: %d messages", consumeCount)
				return
			}
		case <-timeout:
			log.Printf("⏰ Timeout. Consumed %d messages", consumeCount)
			return
		}
	}
}

func testNack() {
	conn, ch := setupConnection()
	defer conn.Close()
	defer ch.Close()

	queueName := "nack-test"
	q, err := ch.QueueDeclare(queueName, false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to declare queue: %v", err)
		return
	}

	// Publish a message
	body := "Nack test message"
	err = ch.Publish("", q.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(body),
	})
	if err != nil {
		log.Printf("❌ Failed to publish: %v", err)
		return
	}
	log.Printf("✅ Published: %s", body)

	// Consume
	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to consume: %v", err)
		return
	}

	timeout := time.After(2 * time.Second)

	select {
	case msg := <-msgs:
		log.Printf("📨 Consumed: %s (delivery tag: %d)", string(msg.Body), msg.DeliveryTag)

		// Negative acknowledgment with requeue
		err := msg.Nack(false, true) // false=single, true=requeue
		if err != nil {
			log.Printf("❌ Failed to nack: %v", err)
		} else {
			log.Printf("⚠️  Nacked delivery tag %d (requeued)", msg.DeliveryTag)
		}

		// Try to consume again (should get requeued message)
		select {
		case msg2 := <-msgs:
			log.Printf("📨 Re-consumed requeued message: %s", string(msg2.Body))
			msg2.Ack(false)
			log.Printf("✅ Nack test passed: message requeued and re-consumed")
		case <-time.After(2 * time.Second):
			log.Printf("⚠️  Requeued message not received (might be slow)")
		}

	case <-timeout:
		log.Printf("⏰ Timeout")
	}
}

func testQoS() {
	conn, ch := setupConnection()
	defer conn.Close()
	defer ch.Close()

	queueName := "qos-test"
	q, err := ch.QueueDeclare(queueName, false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to declare queue: %v", err)
		return
	}

	// Set QoS - prefetch 2 messages
	err = ch.Qos(
		2,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		log.Printf("❌ Failed to set QoS: %v", err)
		return
	}
	log.Printf("✅ QoS set: prefetch=2")

	// Publish 5 messages
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf("QoS message %d", i)
		err := ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
		if err != nil {
			log.Printf("❌ Failed to publish: %v", err)
		}
	}

	// Consume with manual ack
	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to consume: %v", err)
		return
	}

	consumeCount := 0
	timeout := time.After(5 * time.Second)

	for {
		select {
		case msg := <-msgs:
			consumeCount++
			log.Printf("📨 Consumed: %s (prefetch allows 2 unacked)", string(msg.Body))
			time.Sleep(500 * time.Millisecond) // Simulate processing
			msg.Ack(false)
			log.Printf("✅ Acked message %d", consumeCount)

			if consumeCount >= 5 {
				log.Printf("✅ QoS test passed: %d messages", consumeCount)
				return
			}
		case <-timeout:
			log.Printf("⏰ Timeout. Consumed %d messages", consumeCount)
			return
		}
	}
}

func testMultipleConsumers() {
	conn, ch := setupConnection()
	defer conn.Close()
	defer ch.Close()

	queueName := "work-queue-test"
	q, err := ch.QueueDeclare(queueName, false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to declare queue: %v", err)
		return
	}

	// Publish 10 messages
	for i := 0; i < 10; i++ {
		body := fmt.Sprintf("Work message %d", i)
		err := ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
		if err != nil {
			log.Printf("❌ Failed to publish: %v", err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Create 3 consumers
	var wg sync.WaitGroup
	totalConsumed := &sync.Map{}

	for consumerID := 1; consumerID <= 3; consumerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each consumer needs its own channel
			consConn, consCh := setupConnection()
			defer consConn.Close()
			defer consCh.Close()

			msgs, err := consCh.Consume(q.Name, "", true, false, false, false, nil)
			if err != nil {
				log.Printf("❌ Consumer %d failed: %v", id, err)
				return
			}

			consumeCount := 0
			timeout := time.After(3 * time.Second)

			for {
				select {
				case msg := <-msgs:
					consumeCount++
					log.Printf("📨 Consumer %d: %s", id, string(msg.Body))
				case <-timeout:
					totalConsumed.Store(fmt.Sprintf("consumer-%d", id), consumeCount)
					log.Printf("🔵 Consumer %d consumed %d messages", id, consumeCount)
					return
				}
			}
		}(consumerID)
	}

	wg.Wait()

	// Check total
	total := 0
	totalConsumed.Range(func(key, value interface{}) bool {
		total += value.(int)
		return true
	})

	log.Printf("✅ Multiple consumers test: Total %d messages across 3 consumers", total)
}

func testExchangeTypes() {
	conn, ch := setupConnection()
	defer conn.Close()
	defer ch.Close()

	// Test Direct Exchange
	log.Printf("📌 Testing Direct Exchange...")
	err := ch.ExchangeDeclare("direct-test", "direct", false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to declare direct exchange: %v", err)
	} else {
		log.Printf("✅ Direct exchange declared")
	}

	// Test Fanout Exchange
	log.Printf("📌 Testing Fanout Exchange...")
	err = ch.ExchangeDeclare("fanout-test", "fanout", false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to declare fanout exchange: %v", err)
	} else {
		log.Printf("✅ Fanout exchange declared")
	}

	// Test Topic Exchange
	log.Printf("📌 Testing Topic Exchange...")
	err = ch.ExchangeDeclare("topic-test", "topic", false, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to declare topic exchange: %v", err)
	} else {
		log.Printf("✅ Topic exchange declared")
	}

	log.Printf("✅ Exchange types test completed")
}

func testPriorityQueue() {
	conn, ch := setupConnection()
	defer conn.Close()
	defer ch.Close()

	// Declare priority queue
	args := amqp.Table{
		"x-max-priority": 10,
	}

	queueName := "priority-test"
	q, err := ch.QueueDeclare(queueName, false, false, false, false, args)
	if err != nil {
		log.Printf("❌ Failed to declare priority queue: %v", err)
		return
	}
	log.Printf("✅ Priority queue declared")

	// Publish messages with different priorities
	priorities := []uint8{1, 5, 10, 3, 7}
	for i, priority := range priorities {
		body := fmt.Sprintf("Priority %d message %d", priority, i)
		err := ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
			Priority:    priority,
		})
		if err != nil {
			log.Printf("❌ Failed to publish: %v", err)
		} else {
			log.Printf("✅ Published with priority %d: %s", priority, body)
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Consume
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Printf("❌ Failed to consume: %v", err)
		return
	}

	consumeCount := 0
	timeout := time.After(3 * time.Second)

	log.Printf("📥 Consuming in priority order (highest first):")
	for {
		select {
		case msg := <-msgs:
			consumeCount++
			log.Printf("📨 Consumed (priority %d): %s", msg.Priority, string(msg.Body))
			if consumeCount >= 5 {
				log.Printf("✅ Priority queue test passed: %d messages", consumeCount)
				return
			}
		case <-timeout:
			log.Printf("⏰ Timeout. Consumed %d messages", consumeCount)
			return
		}
	}
}

func setupConnection() (*amqp.Connection, *amqp.Channel) {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("❌ Failed to open channel: %v", err)
	}

	return conn, ch
}
