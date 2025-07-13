package queue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

func TestLockFreeQueue(t *testing.T) {
	t.Run("NewLockFreeQueue", func(t *testing.T) {
		queue := NewLockFreeQueue("test-queue", 1024, DropOldest)
		if queue == nil {
			t.Fatal("Expected queue to be created")
		}
		if queue.name != "test-queue" {
			t.Errorf("Expected name 'test-queue', got %s", queue.name)
		}
		if queue.capacity != 1024 {
			t.Errorf("Expected capacity 1024, got %d", queue.capacity)
		}
	})

	t.Run("EnqueueDequeue", func(t *testing.T) {
		queue := NewLockFreeQueue("test-queue", 4, DropOldest)

		message := &types.PortaskMessage{
			ID:        "test-message",
			Topic:     "test-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("test payload"),
		}

		// Test enqueue
		success := queue.Enqueue(message)
		if !success {
			t.Fatal("Failed to enqueue message")
		}

		// Test dequeue
		dequeued, ok := queue.Dequeue()
		if !ok {
			t.Fatal("Failed to dequeue message")
		}
		if dequeued.ID != message.ID {
			t.Errorf("Expected ID %s, got %s", message.ID, dequeued.ID)
		}
	})

	t.Run("EmptyQueue", func(t *testing.T) {
		queue := NewLockFreeQueue("test-queue", 4, DropOldest)

		// Test dequeue from empty queue
		_, ok := queue.Dequeue()
		if ok {
			t.Fatal("Expected dequeue to fail on empty queue")
		}

		if !queue.IsEmpty() {
			t.Error("Expected queue to be empty")
		}
	})

	t.Run("QueueFull", func(t *testing.T) {
		queue := NewLockFreeQueue("test-queue", 2, DropNewest)

		// Fill the queue
		for i := 0; i < 2; i++ {
			message := &types.PortaskMessage{
				ID:        types.MessageID("message-" + string(rune('0'+i))),
				Topic:     "test-topic",
				Timestamp: time.Now().Unix(),
				Payload:   []byte("test payload"),
			}
			success := queue.Enqueue(message)
			if !success {
				t.Fatalf("Failed to enqueue message %d", i)
			}
		}

		// Try to add one more (should be dropped with DropNewest policy)
		message := &types.PortaskMessage{
			ID:        "overflow-message",
			Topic:     "test-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("overflow payload"),
		}
		success := queue.Enqueue(message)
		if success {
			t.Fatal("Expected enqueue to fail due to DropNewest policy")
		}

		stats := queue.Stats()
		if stats.DropCount != 1 {
			t.Errorf("Expected drop count 1, got %d", stats.DropCount)
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		queue := NewLockFreeQueue("test-queue", 1000, DropOldest)

		var wg sync.WaitGroup
		numProducers := 5
		numConsumers := 3
		messagesPerProducer := 100

		// Start producers
		for p := 0; p < numProducers; p++ {
			wg.Add(1)
			go func(producerID int) {
				defer wg.Done()
				for i := 0; i < messagesPerProducer; i++ {
					message := &types.PortaskMessage{
						ID:        types.MessageID("producer-" + string(rune('0'+producerID)) + "-msg-" + string(rune('0'+i))),
						Topic:     "test-topic",
						Timestamp: time.Now().Unix(),
						Payload:   []byte("concurrent test payload"),
					}
					queue.Enqueue(message)
				}
			}(p)
		}

		// Start consumers
		consumedCount := make([]int, numConsumers)
		for c := 0; c < numConsumers; c++ {
			wg.Add(1)
			go func(consumerID int) {
				defer wg.Done()
				for {
					if _, ok := queue.TryDequeue(); ok {
						consumedCount[consumerID]++
					} else {
						time.Sleep(1 * time.Millisecond)
						if queue.IsEmpty() {
							break
						}
					}
				}
			}(c)
		}

		wg.Wait()

		// Check that all messages were consumed
		totalConsumed := 0
		for _, count := range consumedCount {
			totalConsumed += count
		}

		expectedTotal := numProducers * messagesPerProducer
		if totalConsumed != expectedTotal {
			t.Errorf("Expected %d messages consumed, got %d", expectedTotal, totalConsumed)
		}
	})

	t.Run("Statistics", func(t *testing.T) {
		queue := NewLockFreeQueue("test-queue", 10, DropOldest)

		// Add some messages
		for i := 0; i < 5; i++ {
			message := &types.PortaskMessage{
				ID:        types.MessageID("stats-message-" + string(rune('0'+i))),
				Topic:     "test-topic",
				Timestamp: time.Now().Unix(),
				Payload:   []byte("stats test payload"),
			}
			queue.Enqueue(message)
		}

		// Remove some messages
		for i := 0; i < 3; i++ {
			queue.Dequeue()
		}

		stats := queue.Stats()
		if stats.EnqueueCount != 5 {
			t.Errorf("Expected enqueue count 5, got %d", stats.EnqueueCount)
		}
		if stats.DequeueCount != 3 {
			t.Errorf("Expected dequeue count 3, got %d", stats.DequeueCount)
		}
		if stats.Size != 2 {
			t.Errorf("Expected size 2, got %d", stats.Size)
		}
	})
}

func TestMessageBus(t *testing.T) {
	processor := NewDefaultMessageProcessor(nil, nil)

	config := MessageBusConfig{
		HighPriorityQueueSize:   100,
		NormalPriorityQueueSize: 200,
		LowPriorityQueueSize:    100,
		DropPolicy:              DropOldest,
		WorkerPoolConfig: WorkerPoolConfig{
			WorkerCount:      2,
			MessageProcessor: processor,
			BatchSize:        10,
			BatchTimeout:     10 * time.Millisecond,
			EnableProfiling:  false,
		},
		EnableTopicQueues: true,
	}

	t.Run("NewMessageBus", func(t *testing.T) {
		bus := NewMessageBus(config)
		if bus == nil {
			t.Fatal("Expected message bus to be created")
		}
	})

	t.Run("StartStop", func(t *testing.T) {
		bus := NewMessageBus(config)

		err := bus.Start()
		if err != nil {
			t.Fatalf("Failed to start message bus: %v", err)
		}

		err = bus.Stop()
		if err != nil {
			t.Fatalf("Failed to stop message bus: %v", err)
		}
	})

	t.Run("PublishMessage", func(t *testing.T) {
		bus := NewMessageBus(config)
		bus.Start()
		defer bus.Stop()

		message := &types.PortaskMessage{
			ID:        "test-publish",
			Topic:     "test-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("test publish payload"),
		}

		err := bus.Publish(message)
		if err != nil {
			t.Fatalf("Failed to publish message: %v", err)
		}

		// Give some time for processing
		time.Sleep(50 * time.Millisecond)

		stats := bus.GetStats()
		if stats.TotalMessages != 1 {
			t.Errorf("Expected 1 message, got %d", stats.TotalMessages)
		}
	})

	t.Run("PublishToTopic", func(t *testing.T) {
		bus := NewMessageBus(config)
		bus.Start()
		defer bus.Stop()

		message := &types.PortaskMessage{
			ID:        "test-topic-publish",
			Topic:     "special-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("test topic publish payload"),
		}

		err := bus.PublishToTopic(message)
		if err != nil {
			t.Fatalf("Failed to publish to topic: %v", err)
		}

		// Give some time for processing
		time.Sleep(10 * time.Millisecond)

		stats := bus.GetStats()
		if count, exists := stats.TopicStats["special-topic"]; !exists || count != 1 {
			t.Errorf("Expected 1 message for topic 'special-topic', got %d", count)
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		bus := NewMessageBus(config)
		bus.Start()
		defer bus.Stop()

		// Publish several messages
		for i := 0; i < 10; i++ {
			message := &types.PortaskMessage{
				ID:        types.MessageID("stats-test-" + string(rune('0'+i))),
				Topic:     "stats-topic",
				Timestamp: time.Now().Unix(),
				Payload:   []byte("stats test payload"),
			}
			bus.Publish(message)
		}

		// Give time for processing
		time.Sleep(20 * time.Millisecond)

		stats := bus.GetStats()
		if stats.TotalMessages != 10 {
			t.Errorf("Expected 10 total messages, got %d", stats.TotalMessages)
		}

		if len(stats.QueueStats) == 0 {
			t.Error("Expected queue statistics to be populated")
		}

		if len(stats.WorkerStats) != config.WorkerPoolConfig.WorkerCount {
			t.Errorf("Expected %d worker stats, got %d", config.WorkerPoolConfig.WorkerCount, len(stats.WorkerStats))
		}
	})
}

func TestMessageProcessors(t *testing.T) {
	t.Run("DefaultMessageProcessor", func(t *testing.T) {
		processor := NewDefaultMessageProcessor(nil, nil)
		if processor == nil {
			t.Fatal("Expected processor to be created")
		}

		message := &types.PortaskMessage{
			ID:        "test-process",
			Topic:     "test-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("test process payload"),
		}

		ctx := context.Background()
		err := processor.ProcessMessage(ctx, message)
		if err != nil {
			t.Fatalf("Failed to process message: %v", err)
		}
	})

	t.Run("BenchmarkMessageProcessor", func(t *testing.T) {
		processor := NewBenchmarkMessageProcessor()
		if processor == nil {
			t.Fatal("Expected processor to be created")
		}

		message := &types.PortaskMessage{
			ID:        "test-benchmark",
			Topic:     "test-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("test benchmark payload"),
		}

		ctx := context.Background()
		start := time.Now()

		for i := 0; i < 100; i++ {
			err := processor.ProcessMessage(ctx, message)
			if err != nil {
				t.Fatalf("Failed to process message: %v", err)
			}
		}

		duration := time.Since(start)
		count, bytes, totalDuration := processor.GetStats()

		if count != 100 {
			t.Errorf("Expected 100 processed messages, got %d", count)
		}
		if bytes == 0 {
			t.Error("Expected bytes to be counted")
		}
		if totalDuration < duration {
			t.Error("Expected total duration to be at least processing duration")
		}
	})

	t.Run("EchoMessageProcessor", func(t *testing.T) {
		outputChan := make(chan *types.PortaskMessage, 10)
		processor := NewEchoMessageProcessor(outputChan)

		message := &types.PortaskMessage{
			ID:        "test-echo",
			Topic:     "test-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("test echo payload"),
		}

		ctx := context.Background()
		err := processor.ProcessMessage(ctx, message)
		if err != nil {
			t.Fatalf("Failed to process message: %v", err)
		}

		select {
		case echoed := <-outputChan:
			if echoed.ID != message.ID {
				t.Errorf("Expected echoed message ID %s, got %s", message.ID, echoed.ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected message to be echoed")
		}
	})

	t.Run("ChainedMessageProcessor", func(t *testing.T) {
		processor1 := NewDefaultMessageProcessor(nil, nil)
		processor2 := NewDefaultMessageProcessor(nil, nil)

		chained := NewChainedMessageProcessor(processor1, processor2)

		message := &types.PortaskMessage{
			ID:        "test-chained",
			Topic:     "test-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("test chained payload"),
		}

		ctx := context.Background()
		err := chained.ProcessMessage(ctx, message)
		if err != nil {
			t.Fatalf("Failed to process chained message: %v", err)
		}
	})

	t.Run("FilterMessageProcessor", func(t *testing.T) {
		baseProcessor := NewDefaultMessageProcessor(nil, nil)

		// Filter that only allows messages with "allowed" in the topic
		filter := func(msg *types.PortaskMessage) bool {
			return msg.Topic == "allowed-topic"
		}

		processor := NewFilterMessageProcessor(filter, baseProcessor)

		allowedMessage := &types.PortaskMessage{
			ID:        "test-allowed",
			Topic:     "allowed-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("allowed payload"),
		}

		filteredMessage := &types.PortaskMessage{
			ID:        "test-filtered",
			Topic:     "filtered-topic",
			Timestamp: time.Now().Unix(),
			Payload:   []byte("filtered payload"),
		}

		ctx := context.Background()

		// Should process allowed message
		err := processor.ProcessMessage(ctx, allowedMessage)
		if err != nil {
			t.Fatalf("Failed to process allowed message: %v", err)
		}

		// Should skip filtered message (no error)
		err = processor.ProcessMessage(ctx, filteredMessage)
		if err != nil {
			t.Fatalf("Filter processor should not error on filtered messages: %v", err)
		}
	})
}

// Benchmark tests
func BenchmarkLockFreeQueue(b *testing.B) {
	queue := NewLockFreeQueue("benchmark-queue", 10000, DropOldest)

	message := &types.PortaskMessage{
		ID:        "bench-message",
		Topic:     "benchmark",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("benchmark payload"),
	}

	b.Run("Enqueue", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			queue.Enqueue(message)
		}
	})

	// Fill queue for dequeue benchmark
	for i := 0; i < 1000; i++ {
		queue.Enqueue(message)
	}

	b.Run("Dequeue", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			queue.Dequeue()
		}
	})
}

func BenchmarkMessageBusThroughput(b *testing.B) {
	processor := NewBenchmarkMessageProcessor()

	config := MessageBusConfig{
		HighPriorityQueueSize:   10000,
		NormalPriorityQueueSize: 10000,
		LowPriorityQueueSize:    10000,
		DropPolicy:              DropOldest,
		WorkerPoolConfig: WorkerPoolConfig{
			WorkerCount:      4,
			MessageProcessor: processor,
			BatchSize:        50,
			BatchTimeout:     1 * time.Millisecond,
			EnableProfiling:  false,
		},
		EnableTopicQueues: false,
	}

	bus := NewMessageBus(config)
	bus.Start()
	defer bus.Stop()

	message := &types.PortaskMessage{
		ID:        "bench-message",
		Topic:     "benchmark",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("benchmark throughput payload"),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bus.Publish(message)
		}
	})

	// Wait for processing to complete
	time.Sleep(100 * time.Millisecond)

	count, bytes, duration := processor.GetStats()
	messagesPerSecond := float64(count) / duration.Seconds()
	mbPerSecond := float64(bytes) / 1024 / 1024 / duration.Seconds()

	b.ReportMetric(messagesPerSecond, "msg/sec")
	b.ReportMetric(mbPerSecond, "MB/sec")
}
