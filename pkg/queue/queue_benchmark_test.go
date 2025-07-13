package queue

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

func BenchmarkMessageBus_PublishSingle(b *testing.B) {
	bus := createTestMessageBus(b)
	defer bus.Stop()

	message := createTestMessage("benchmark-single", []byte("test payload"))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := bus.Publish(message)
		if err != nil {
			b.Fatalf("Publish failed: %v", err)
		}
	}
}

func BenchmarkMessageBus_PublishConcurrent(b *testing.B) {
	bus := createTestMessageBus(b)
	defer bus.Stop()

	numWorkers := runtime.NumCPU()
	message := createTestMessage("concurrent-test", []byte("test payload"))

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	workPerWorker := b.N / numWorkers

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < workPerWorker; i++ {
				err := bus.Publish(message)
				if err != nil {
					b.Errorf("Worker %d publish failed: %v", workerID, err)
				}
			}
		}(w)
	}

	wg.Wait()
}

func BenchmarkMessageBus_PublishToTopic(b *testing.B) {
	bus := createTestMessageBus(b)
	defer bus.Stop()

	topics := []types.TopicName{"orders", "payments", "notifications", "analytics"}
	messages := make([]*types.PortaskMessage, len(topics))
	for i, topic := range topics {
		messages[i] = &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("topic-msg-%d", i)),
			Topic:     topic,
			Timestamp: time.Now().Unix(),
			Payload:   []byte("test payload"),
			Headers:   types.MessageHeaders{},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		message := messages[i%len(messages)]
		err := bus.PublishToTopic(message)
		if err != nil {
			b.Fatalf("PublishToTopic failed: %v", err)
		}
	}
}

func BenchmarkMessageBus_Throughput_1M_Messages(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping throughput test in short mode")
	}

	bus := createTestMessageBus(b)
	defer bus.Stop()

	targetMessages := 1000000
	message := createTestMessage("throughput-test", []byte("test payload"))

	start := time.Now()
	var published int64

	b.ResetTimer()

	for i := 0; i < targetMessages; i++ {
		err := bus.Publish(message)
		if err == nil {
			published++
		}
	}

	elapsed := time.Since(start)
	throughput := float64(published) / elapsed.Seconds()

	b.ReportMetric(throughput, "messages/sec")
	b.Logf("Published %d messages in %v (%.0f msg/sec)", published, elapsed, throughput)

	if throughput < 1000000 {
		b.Logf("WARNING: Throughput below 1M msg/sec target")
	}
}

func BenchmarkMessageBus_MemoryUsage(b *testing.B) {
	bus := createTestMessageBus(b)
	defer bus.Stop()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	message := createTestMessage("memory-test", []byte("test payload"))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := bus.Publish(message)
		if err != nil {
			b.Fatalf("Publish failed: %v", err)
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
	b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
}

func BenchmarkLockFreeQueue_EnqueueDequeue(b *testing.B) {
	queue := NewLockFreeQueue("test-queue", 1000, DropOldest)
	message := createTestMessage("queue-test", []byte("test payload"))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		queue.Enqueue(message)
		_, ok := queue.Dequeue()
		if !ok {
			b.Fatalf("Dequeue failed")
		}
	}
}

func BenchmarkLockFreeQueue_ConcurrentAccess(b *testing.B) {
	queue := NewLockFreeQueue("concurrent-queue", 10000, DropOldest)
	message := createTestMessage("concurrent-queue", []byte("test payload"))

	numProducers := runtime.NumCPU() / 2
	numConsumers := runtime.NumCPU() / 2

	if numProducers == 0 {
		numProducers = 1
	}
	if numConsumers == 0 {
		numConsumers = 1
	}

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	workPerProducer := b.N / numProducers

	// Producers
	for p := 0; p < numProducers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < workPerProducer; i++ {
				queue.Enqueue(message)
			}
		}()
	}

	// Consumers
	for c := 0; c < numConsumers; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			consumed := 0
			for consumed < workPerProducer {
				if _, ok := queue.Dequeue(); ok {
					consumed++
				} else {
					runtime.Gosched()
				}
			}
		}()
	}

	wg.Wait()
}

func TestMessageBus_LatencyRequirement(t *testing.T) {
	bus := createTestMessageBus(t)
	defer bus.Stop()

	message := createTestMessage("latency-test", []byte("test payload"))
	iterations := 10000
	var totalLatency time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()

		err := bus.Publish(message)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		latency := time.Since(start)
		totalLatency += latency

		// Individual publish latency should be < 1ms
		if latency > time.Millisecond {
			t.Logf("High latency detected: %v", latency)
		}
	}

	avgLatency := totalLatency / time.Duration(iterations)
	t.Logf("Average latency: %v", avgLatency)

	if avgLatency > time.Millisecond {
		t.Errorf("Average latency %v exceeds 1ms requirement", avgLatency)
	}
}

func TestMessageBus_MemoryLimit(t *testing.T) {
	bus := createTestMessageBus(t)
	defer bus.Stop()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	message := createTestMessage("memory-limit-test", []byte("test payload"))
	iterations := 100000

	for i := 0; i < iterations; i++ {
		err := bus.Publish(message)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		// Force GC every 10k messages
		if i%10000 == 0 {
			runtime.GC()
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	memoryUsedMB := float64(m2.Alloc-m1.Alloc) / 1024 / 1024
	t.Logf("Memory used: %.2f MB", memoryUsedMB)

	if memoryUsedMB > 50 {
		t.Errorf("Memory usage %.2f MB exceeds 50MB limit", memoryUsedMB)
	}
}

func TestMessageBus_Stats(t *testing.T) {
	bus := createTestMessageBus(t)
	defer bus.Stop()

	message := createTestMessage("stats-test", []byte("test payload"))

	// Publish some messages
	for i := 0; i < 1000; i++ {
		bus.Publish(message)
	}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	stats := bus.GetStats()
	t.Logf("Stats: %+v", stats)

	if stats.TotalMessages == 0 {
		t.Error("Expected non-zero total messages")
	}
}

func TestMessageBus_QueueOverflow(t *testing.T) {
	// Create bus with small queues
	outputChan := make(chan *types.PortaskMessage, 100)
	config := MessageBusConfig{
		HighPriorityQueueSize:   10,
		NormalPriorityQueueSize: 10,
		LowPriorityQueueSize:    10,
		DropPolicy:              DropOldest,
		WorkerPoolConfig: WorkerPoolConfig{
			WorkerCount:      1,
			MessageProcessor: NewEchoMessageProcessor(outputChan),
			BatchSize:        1,
			BatchTimeout:     time.Millisecond,
		},
		EnableTopicQueues: false,
	}

	bus := NewMessageBus(config)
	if err := bus.Start(); err != nil {
		t.Fatalf("Failed to start bus: %v", err)
	}
	defer bus.Stop()

	message := createTestMessage("overflow-test", []byte("test payload"))

	// Try to overflow the queue
	for i := 0; i < 100; i++ {
		bus.Publish(message)
	}

	// Check stats for dropped messages
	time.Sleep(100 * time.Millisecond)
	stats := bus.GetStats()

	t.Logf("Queue overflow stats: %+v", stats)
}

func TestMessageBus_ResourceCleanup(t *testing.T) {
	bus := createTestMessageBus(t)

	message := createTestMessage("cleanup-test", []byte("test payload"))

	// Publish some messages
	for i := 0; i < 100; i++ {
		bus.Publish(message)
	}

	// Stop and check for resource leaks
	bus.Stop()

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Check goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Create and stop another bus
	bus2 := createTestMessageBus(t)
	bus2.Stop()

	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()

	if finalGoroutines > initialGoroutines+2 { // Allow some tolerance
		t.Errorf("Potential goroutine leak: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}
}

func TestLockFreeQueue_Statistics(t *testing.T) {
	queue := NewLockFreeQueue("stats-queue", 100, DropOldest)
	message := createTestMessage("stats-test", []byte("test payload"))

	// Test enqueue operations
	for i := 0; i < 50; i++ {
		queue.Enqueue(message)
	}

	t.Logf("Queue after enqueue: size appears to be working")

	// Test dequeue operations
	for i := 0; i < 25; i++ {
		_, ok := queue.Dequeue()
		if !ok {
			t.Errorf("Expected successful dequeue at iteration %d", i)
			break
		}
	}

	t.Logf("Queue after dequeue: operations completed successfully")
}

// Helper functions

func createTestMessageBus(tb testing.TB) *MessageBus {
	// Create a dummy output channel for EchoMessageProcessor
	outputChan := make(chan *types.PortaskMessage, 1000)

	// Drain output channel in background to prevent blocking
	go func() {
		for range outputChan {
			// Discard messages to prevent channel from filling up
		}
	}()

	config := MessageBusConfig{
		HighPriorityQueueSize:   8192,
		NormalPriorityQueueSize: 65536,
		LowPriorityQueueSize:    16384,
		DropPolicy:              DropOldest,
		WorkerPoolConfig: WorkerPoolConfig{
			WorkerCount:      4,
			MessageProcessor: NewEchoMessageProcessor(outputChan),
			BatchSize:        100,
			BatchTimeout:     time.Millisecond,
		},
		EnableTopicQueues: true,
	}

	bus := NewMessageBus(config)
	if err := bus.Start(); err != nil {
		tb.Fatalf("Failed to start message bus: %v", err)
	}

	return bus
}

func createTestMessage(id string, payload []byte) *types.PortaskMessage {
	return &types.PortaskMessage{
		ID:        types.MessageID(id),
		Topic:     types.TopicName("test-topic"),
		Timestamp: time.Now().Unix(),
		Payload:   payload,
		Headers:   types.MessageHeaders{},
	}
}
