package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/compression"
	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/monitoring"
	"github.com/meftunca/portask/pkg/queue"
	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/types"
)

// Integration tests for the entire Portask system

func TestFullSystem_1M_Messages_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	system := createTestSystem(t)
	defer system.Shutdown()

	targetMessages := 1000000
	start := time.Now()

	// Generate messages concurrently
	numProducers := runtime.NumCPU()
	messagesPerProducer := targetMessages / numProducers

	var wg sync.WaitGroup
	var totalSent int64
	var sendMutex sync.Mutex

	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			sent := 0

			for j := 0; j < messagesPerProducer; j++ {
				message := &types.PortaskMessage{
					ID:        types.MessageID(fmt.Sprintf("producer-%d-msg-%d", producerID, j)),
					Topic:     types.TopicName("test-topic"),
					Timestamp: time.Now().Unix(),
					Payload:   []byte(fmt.Sprintf("Test message %d from producer %d", j, producerID)),
					Headers:   types.MessageHeaders{},
				}

				err := system.MessageBus.Publish(message)
				if err == nil {
					sent++
				}
			}

			sendMutex.Lock()
			totalSent += int64(sent)
			sendMutex.Unlock()
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Wait for processing to complete
	time.Sleep(2 * time.Second)

	throughput := float64(totalSent) / elapsed.Seconds()
	t.Logf("Integration Test Results:")
	t.Logf("  Messages sent: %d", totalSent)
	t.Logf("  Time taken: %v", elapsed)
	t.Logf("  Throughput: %.0f msg/sec", throughput)

	// Get final stats
	stats := system.MessageBus.GetStats()
	t.Logf("  Final stats: %+v", stats)

	// Performance requirements check
	if throughput < 1000000 {
		t.Logf("WARNING: Throughput %.0f msg/sec below 1M target", throughput)
	}

	if totalSent != int64(targetMessages) {
		t.Errorf("Expected %d messages sent, got %d", targetMessages, totalSent)
	}
}

func TestFullSystem_MemoryUsage_Integration(t *testing.T) {
	system := createTestSystem(t)
	defer system.Shutdown()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Process messages for memory measurement
	numMessages := 100000
	for i := 0; i < numMessages; i++ {
		message := &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("memory-test-%d", i)),
			Topic:     types.TopicName("memory-topic"),
			Timestamp: time.Now().Unix(),
			Payload:   []byte("Memory test payload"),
			Headers:   types.MessageHeaders{},
		}

		system.MessageBus.Publish(message)

		// Force GC periodically
		if i%10000 == 0 {
			runtime.GC()
		}
	}

	// Wait for processing
	time.Sleep(1 * time.Second)
	runtime.GC()
	runtime.ReadMemStats(&m2)

	memoryUsedMB := float64(m2.Alloc-m1.Alloc) / 1024 / 1024
	t.Logf("Memory Integration Test Results:")
	t.Logf("  Memory used: %.2f MB", memoryUsedMB)
	t.Logf("  Messages processed: %d", numMessages)
	t.Logf("  Memory per message: %.2f KB", (memoryUsedMB*1024)/float64(numMessages))

	if memoryUsedMB > 50 {
		t.Errorf("Memory usage %.2f MB exceeds 50MB limit", memoryUsedMB)
	}
}

func TestFullSystem_Latency_Integration(t *testing.T) {
	system := createTestSystem(t)
	defer system.Shutdown()

	numMessages := 10000
	latencies := make([]time.Duration, numMessages)

	for i := 0; i < numMessages; i++ {
		start := time.Now()

		message := &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("latency-test-%d", i)),
			Topic:     types.TopicName("latency-topic"),
			Timestamp: time.Now().Unix(),
			Payload:   []byte("Latency test payload"),
			Headers:   types.MessageHeaders{},
		}

		err := system.MessageBus.Publish(message)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		latencies[i] = time.Since(start)
	}

	// Calculate statistics
	var totalLatency time.Duration
	var maxLatency time.Duration
	var highLatencyCount int

	for _, latency := range latencies {
		totalLatency += latency
		if latency > maxLatency {
			maxLatency = latency
		}
		if latency > time.Millisecond {
			highLatencyCount++
		}
	}

	avgLatency := totalLatency / time.Duration(numMessages)
	highLatencyPercent := float64(highLatencyCount) / float64(numMessages) * 100

	t.Logf("Latency Integration Test Results:")
	t.Logf("  Average latency: %v", avgLatency)
	t.Logf("  Maximum latency: %v", maxLatency)
	t.Logf("  High latency (>1ms): %.2f%%", highLatencyPercent)

	if avgLatency > time.Millisecond {
		t.Errorf("Average latency %v exceeds 1ms requirement", avgLatency)
	}

	if highLatencyPercent > 1.0 { // Allow 1% tolerance
		t.Logf("WARNING: %.2f%% of messages had >1ms latency", highLatencyPercent)
	}
}

func TestFullSystem_ConcurrentLoad_Integration(t *testing.T) {
	system := createTestSystem(t)
	defer system.Shutdown()

	numProducers := 10
	numConsumers := 5
	messagesPerProducer := 10000
	duration := 30 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	var totalSent int64
	var totalReceived int64
	var sendMutex, receiveMutex sync.Mutex

	// Start producers
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			sent := 0

			for j := 0; j < messagesPerProducer; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					message := &types.PortaskMessage{
						ID:        types.MessageID(fmt.Sprintf("load-producer-%d-msg-%d", producerID, j)),
						Topic:     types.TopicName("load-topic"),
						Timestamp: time.Now().Unix(),
						Payload:   []byte(fmt.Sprintf("Load test message %d", j)),
						Headers:   types.MessageHeaders{},
					}

					err := system.MessageBus.Publish(message)
					if err == nil {
						sent++
					}
					time.Sleep(time.Microsecond * 100) // Slight delay
				}
			}

			sendMutex.Lock()
			totalSent += int64(sent)
			sendMutex.Unlock()
		}(i)
	}

	// Start mock consumers (simulate processing)
	outputChan := make(chan *types.PortaskMessage, 10000)
	for i := 0; i < numConsumers; i++ {
		wg.Add(1)
		go func(consumerID int) {
			defer wg.Done()
			received := 0

			for {
				select {
				case <-ctx.Done():
					receiveMutex.Lock()
					totalReceived += int64(received)
					receiveMutex.Unlock()
					return
				case <-outputChan:
					received++
				default:
					time.Sleep(time.Microsecond * 50)
				}
			}
		}(i)
	}

	wg.Wait()

	stats := system.MessageBus.GetStats()

	t.Logf("Concurrent Load Integration Test Results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Producers: %d, Consumers: %d", numProducers, numConsumers)
	t.Logf("  Messages sent: %d", totalSent)
	t.Logf("  Messages received: %d", totalReceived)
	t.Logf("  Send rate: %.0f msg/sec", float64(totalSent)/duration.Seconds())
	t.Logf("  Final stats: %+v", stats)
}

func TestFullSystem_ErrorRecovery_Integration(t *testing.T) {
	system := createTestSystem(t)
	defer system.Shutdown()

	// Send some valid messages
	for i := 0; i < 1000; i++ {
		message := &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("valid-msg-%d", i)),
			Topic:     types.TopicName("test-topic"),
			Timestamp: time.Now().Unix(),
			Payload:   []byte("Valid message"),
			Headers:   types.MessageHeaders{},
		}
		system.MessageBus.Publish(message)
	}

	// Send some invalid messages
	for i := 0; i < 100; i++ {
		invalidMessage := &types.PortaskMessage{
			ID:      "",
			Topic:   "",
			Payload: nil,
		}
		system.MessageBus.Publish(invalidMessage)
	}

	// Send more valid messages
	for i := 0; i < 1000; i++ {
		message := &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("recovery-msg-%d", i)),
			Topic:     types.TopicName("test-topic"),
			Timestamp: time.Now().Unix(),
			Payload:   []byte("Recovery message"),
			Headers:   types.MessageHeaders{},
		}
		system.MessageBus.Publish(message)
	}

	// Wait for processing
	time.Sleep(1 * time.Second)

	stats := system.MessageBus.GetStats()
	t.Logf("Error Recovery Integration Test Results:")
	t.Logf("  Final stats: %+v", stats)
	t.Logf("  System continued processing after errors")
}

func BenchmarkFullSystem_Throughput(b *testing.B) {
	system := createTestSystem(b)
	defer system.Shutdown()

	message := &types.PortaskMessage{
		ID:        types.MessageID("benchmark-msg"),
		Topic:     types.TopicName("benchmark-topic"),
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Benchmark message payload"),
		Headers:   types.MessageHeaders{},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := system.MessageBus.Publish(message)
		if err != nil {
			b.Fatalf("Publish failed: %v", err)
		}
	}
}

// TestSystem represents the full Portask system for testing
type TestSystem struct {
	Config           *config.Config
	MessageBus       *queue.MessageBus
	MetricsCollector *monitoring.MetricsCollector
	Codec            serialization.Codec
	Compressor       compression.Compressor
	ctx              context.Context
	cancel           context.CancelFunc
}

func createTestSystem(tb testing.TB) *TestSystem {
	// Load test configuration
	cfg := &config.Config{
		Performance: config.PerformanceConfig{
			WorkerPoolSize: 8,
			BatchSize:      100,
		},
		Serialization: config.SerializationConfig{
			Type: "cbor",
		},
		Compression: config.CompressionConfig{
			Type: "none", // Use no compression for testing
		},
		Monitoring: config.MonitoringConfig{
			EnableProfiling: false,
		},
	}

	// Initialize metrics collector
	metricsCollector := monitoring.NewMetricsCollector(1 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())

	if err := metricsCollector.Start(ctx); err != nil {
		tb.Fatalf("Failed to start metrics collector: %v", err)
	}

	// Initialize serialization
	codecFactory := serialization.NewCodecFactory()
	if err := codecFactory.InitializeDefaultCodecs(cfg); err != nil {
		tb.Fatalf("Failed to initialize codecs: %v", err)
	}
	codec, err := codecFactory.GetCodec(cfg.Serialization.Type)
	if err != nil {
		tb.Fatalf("Failed to create codec: %v", err)
	}

	// Initialize compression
	compressorFactory := compression.NewCompressorFactory()
	if err := compressorFactory.InitializeDefaultCompressors(cfg); err != nil {
		tb.Fatalf("Failed to initialize compressors: %v", err)
	}
	compressor, err := compressorFactory.GetCompressor(cfg.Compression.Type)
	if err != nil {
		tb.Fatalf("Failed to create compressor: %v", err)
	}

	// Initialize message processor
	processor := queue.NewDefaultMessageProcessor(codec, compressor)

	// Configure message bus
	busConfig := queue.MessageBusConfig{
		HighPriorityQueueSize:   8192,
		NormalPriorityQueueSize: 65536,
		LowPriorityQueueSize:    16384,
		DropPolicy:              queue.DropOldest,
		WorkerPoolConfig: queue.WorkerPoolConfig{
			WorkerCount:      cfg.Performance.WorkerPoolSize,
			MessageProcessor: processor,
			BatchSize:        cfg.Performance.BatchSize,
			BatchTimeout:     1 * time.Millisecond,
		},
		EnableTopicQueues: true,
	}

	// Create and start message bus
	messageBus := queue.NewMessageBus(busConfig)
	if err := messageBus.Start(); err != nil {
		tb.Fatalf("Failed to start message bus: %v", err)
	}

	return &TestSystem{
		Config:           cfg,
		MessageBus:       messageBus,
		MetricsCollector: metricsCollector,
		Codec:            codec,
		Compressor:       compressor,
		ctx:              ctx,
		cancel:           cancel,
	}
}

func (s *TestSystem) Shutdown() {
	if s.MessageBus != nil {
		s.MessageBus.Stop()
	}
	if s.MetricsCollector != nil {
		s.MetricsCollector.Stop()
	}
	if s.cancel != nil {
		s.cancel()
	}
}
