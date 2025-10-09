package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

// Comprehensive Kafka Consumer Group Test using kafka-go library
func main() {
	log.Printf("🧪 Testing Kafka Consumer Groups with kafka-go library...")

	// Test 1: Single Consumer
	log.Printf("\n📝 Test 1: Single Consumer")
	testSingleConsumer()

	// Test 2: Consumer Group with Multiple Consumers
	log.Printf("\n📝 Test 2: Consumer Group (3 consumers)")
	testConsumerGroup()

	// Test 3: Manual Offset Commit
	log.Printf("\n📝 Test 3: Manual Offset Commit")
	testManualCommit()

	// Test 4: Seek to Specific Offset
	log.Printf("\n📝 Test 4: Seek to Specific Offset")
	testSeek()

	// Test 5: Consumer Lag Monitoring
	log.Printf("\n📝 Test 5: Consumer Lag")
	testConsumerLag()

	log.Printf("\n✅ All Kafka consumer tests completed!")
}

func testSingleConsumer() {
	// Create producer
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "single-consumer-test",
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// Produce messages
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		err := writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(fmt.Sprintf("key-%d", i)),
			Value: []byte(fmt.Sprintf("Message %d", i)),
		})
		if err != nil {
			log.Printf("❌ Failed to produce: %v", err)
		} else {
			log.Printf("✅ Produced message %d", i)
		}
	}

	// Create consumer
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "single-consumer-test",
		GroupID:  "single-consumer-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	// Consume messages
	consumeCount := 0
	timeout := time.After(5 * time.Second)
	for consumeCount < 5 {
		select {
		case <-timeout:
			log.Printf("⏰ Timeout reached. Consumed %d/5 messages", consumeCount)
			return
		default:
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			msg, err := reader.ReadMessage(ctx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					continue
				}
				log.Printf("❌ Error reading: %v", err)
				return
			}

			consumeCount++
			log.Printf("📨 Consumed: %s (offset: %d)", string(msg.Value), msg.Offset)
		}
	}

	log.Printf("✅ Single consumer test passed: %d/5 messages", consumeCount)
}

func testConsumerGroup() {
	// Create producer
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "group-consumer-test",
		Balancer: &kafka.Hash{}, // Hash balancer for partition distribution
	}
	defer writer.Close()

	// Produce 15 messages
	ctx := context.Background()
	for i := 0; i < 15; i++ {
		err := writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(fmt.Sprintf("key-%d", i)),
			Value: []byte(fmt.Sprintf("Group message %d", i)),
		})
		if err != nil {
			log.Printf("❌ Failed to produce: %v", err)
		} else {
			log.Printf("✅ Produced group message %d", i)
		}
	}

	time.Sleep(500 * time.Millisecond) // Wait for messages to be available

	// Create 3 consumers in same group
	var wg sync.WaitGroup
	totalConsumed := &sync.Map{}

	for consumerID := 1; consumerID <= 3; consumerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			reader := kafka.NewReader(kafka.ReaderConfig{
				Brokers:  []string{"localhost:9092"},
				Topic:    "group-consumer-test",
				GroupID:  "multi-consumer-group",
				MinBytes: 1,
				MaxBytes: 10e6,
			})
			defer reader.Close()

			consumeCount := 0
			timeout := time.After(5 * time.Second)

			for {
				select {
				case <-timeout:
					totalConsumed.Store(fmt.Sprintf("consumer-%d", id), consumeCount)
					log.Printf("🔵 Consumer %d consumed %d messages", id, consumeCount)
					return
				default:
					ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
					msg, err := reader.ReadMessage(ctx)
					cancel()

					if err != nil {
						if err == context.DeadlineExceeded {
							continue
						}
						log.Printf("❌ Consumer %d error: %v", id, err)
						return
					}

					consumeCount++
					log.Printf("📨 Consumer %d: %s (offset: %d, partition: %d)",
						id, string(msg.Value), msg.Offset, msg.Partition)
				}
			}
		}(consumerID)
	}

	wg.Wait()

	// Check total consumed
	total := 0
	totalConsumed.Range(func(key, value interface{}) bool {
		total += value.(int)
		return true
	})

	log.Printf("✅ Consumer group test: Total consumed %d messages across 3 consumers", total)
}

func testManualCommit() {
	// Create producer
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "manual-commit-test",
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// Produce messages
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		err := writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(fmt.Sprintf("key-%d", i)),
			Value: []byte(fmt.Sprintf("Commit message %d", i)),
		})
		if err != nil {
			log.Printf("❌ Failed to produce: %v", err)
		}
	}

	// Create consumer with manual commit
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "manual-commit-test",
		GroupID:  "manual-commit-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	// Consume and manually commit
	consumeCount := 0
	timeout := time.After(5 * time.Second)

	for consumeCount < 5 {
		select {
		case <-timeout:
			log.Printf("⏰ Timeout reached")
			return
		default:
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			msg, err := reader.FetchMessage(ctx) // FetchMessage doesn't auto-commit
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					continue
				}
				log.Printf("❌ Error fetching: %v", err)
				return
			}

			consumeCount++
			log.Printf("📨 Fetched: %s (offset: %d)", string(msg.Value), msg.Offset)

			// Manual commit
			ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
			err = reader.CommitMessages(ctx2, msg)
			cancel2()

			if err != nil {
				log.Printf("❌ Failed to commit offset %d: %v", msg.Offset, err)
			} else {
				log.Printf("✅ Committed offset %d", msg.Offset)
			}
		}
	}

	log.Printf("✅ Manual commit test passed: %d messages", consumeCount)
}

func testSeek() {
	// Create producer
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "seek-test",
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// Produce messages
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		err := writer.WriteMessages(ctx, kafka.Message{
			Value: []byte(fmt.Sprintf("Seek message %d", i)),
		})
		if err != nil {
			log.Printf("❌ Failed to produce: %v", err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Create consumer
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "seek-test",
		GroupID:  "seek-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	// Seek to offset 5
	log.Printf("🔍 Seeking to offset 5...")
	err := reader.SetOffset(5)
	if err != nil {
		log.Printf("❌ Failed to seek: %v", err)
		return
	}

	// Read from offset 5
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	msg, err := reader.ReadMessage(ctx2)
	cancel2()

	if err != nil {
		log.Printf("❌ Failed to read after seek: %v", err)
		return
	}

	log.Printf("📨 First message after seek: %s (offset: %d)", string(msg.Value), msg.Offset)

	if msg.Offset >= 5 {
		log.Printf("✅ Seek test passed: offset %d >= 5", msg.Offset)
	} else {
		log.Printf("❌ Seek test failed: expected offset >= 5, got %d", msg.Offset)
	}
}

func testConsumerLag() {
	// Create producer
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "lag-test",
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// Produce 20 messages
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		err := writer.WriteMessages(ctx, kafka.Message{
			Value: []byte(fmt.Sprintf("Lag message %d", i)),
		})
		if err != nil {
			log.Printf("❌ Failed to produce: %v", err)
		}
	}

	// Create slow consumer (consumes only 5 messages)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "lag-test",
		GroupID:  "lag-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	// Consume only 5 messages (leaving 15 behind = lag)
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		msg, err := reader.ReadMessage(ctx)
		cancel()

		if err != nil {
			log.Printf("❌ Error reading: %v", err)
			break
		}

		log.Printf("📨 Consumed: %s (offset: %d)", string(msg.Value), msg.Offset)
	}

	// Check lag via Stats
	stats := reader.Stats()
	log.Printf("📊 Consumer Stats:")
	log.Printf("   - Messages: %d", stats.Messages)
	log.Printf("   - Bytes: %d", stats.Bytes)
	log.Printf("   - Offset: %d", stats.Offset)
	log.Printf("   - Lag: %d", stats.Lag)

	if stats.Lag > 0 {
		log.Printf("✅ Consumer lag detected: %d messages behind", stats.Lag)
	} else {
		log.Printf("⚠️  No lag detected (might be normal if messages consumed quickly)")
	}
}

// Helper function for graceful shutdown
func setupGracefulShutdown() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Printf("\n🛑 Graceful shutdown initiated...")
		cancel()
	}()

	return ctx
}
