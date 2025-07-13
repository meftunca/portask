package main

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/compression"
	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/queue"
	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/types"
)

// TestPortaskPhase1 tests the core Phase 1 implementation
func TestPortaskPhase1(t *testing.T) {
	log.Printf("🧪 Testing Portask Phase 1 Implementation")

	// Test configuration loading
	cfg := config.DefaultConfig()
	if cfg == nil {
		t.Fatal("Failed to create default config")
	}
	log.Printf("✅ Configuration: OK")

	// Test serialization
	codecFactory := serialization.NewCodecFactory()
	if err := codecFactory.InitializeDefaultCodecs(cfg); err != nil {
		t.Fatalf("Failed to initialize codecs: %v", err)
	}
	codec, err := codecFactory.GetCodec(cfg.Serialization.Type)
	if err != nil {
		t.Fatalf("Failed to create codec: %v", err)
	}
	log.Printf("✅ Serialization (%s): OK", cfg.Serialization.Type)

	// Test compression
	compressorFactory := compression.NewCompressorFactory()
	if err := compressorFactory.InitializeDefaultCompressors(cfg); err != nil {
		t.Fatalf("Failed to initialize compressors: %v", err)
	}
	compressor, err := compressorFactory.GetCompressor(cfg.Compression.Type)
	if err != nil {
		t.Fatalf("Failed to create compressor: %v", err)
	}
	log.Printf("✅ Compression (%s): OK", cfg.Compression.Type)

	// Test message creation and serialization
	testMessage := &types.PortaskMessage{
		ID:        "test-message-001",
		Topic:     "test-topic",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("This is a test message payload for serialization testing"),
	}

	// Test serialization
	serialized, err := codec.Encode(testMessage)
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}

	var deserialized, err2 = codec.Decode(serialized)
	if err2 != nil {
		t.Fatalf("Failed to deserialize message: %v", err2)
	}

	if deserialized.ID != testMessage.ID || string(deserialized.Payload) != string(testMessage.Payload) {
		t.Fatal("Serialization/deserialization mismatch")
	}
	log.Printf("✅ Message Serialization: OK")

	// Test compression
	compressed, err := compressor.Compress(serialized)
	if err != nil {
		t.Fatalf("Failed to compress data: %v", err)
	}

	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("Failed to decompress data: %v", err)
	}

	if string(decompressed) != string(serialized) {
		t.Fatal("Compression/decompression mismatch")
	}
	log.Printf("✅ Message Compression: OK")

	// Test message bus
	processor := queue.NewDefaultMessageProcessor(codec, compressor)

	busConfig := queue.MessageBusConfig{
		HighPriorityQueueSize:   1024,
		NormalPriorityQueueSize: 1024,
		LowPriorityQueueSize:    1024,
		DropPolicy:              queue.DropOldest,
		WorkerPoolConfig: queue.WorkerPoolConfig{
			WorkerCount:      2,
			MessageProcessor: processor,
			BatchSize:        10,
			BatchTimeout:     10 * time.Millisecond,
			EnableProfiling:  false,
		},
		EnableTopicQueues: true,
	}

	messageBus := queue.NewMessageBus(busConfig)
	if err := messageBus.Start(); err != nil {
		t.Fatalf("Failed to start message bus: %v", err)
	}
	defer messageBus.Stop()

	log.Printf("✅ Message Bus: OK")

	// Test message publishing
	testMessages := 1000
	for i := 0; i < testMessages; i++ {
		msg := &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("perf-test-%d", i)),
			Topic:     "performance-test",
			Timestamp: time.Now().Unix(),
			Payload:   []byte(fmt.Sprintf("Performance test message #%d", i)),
		}

		if err := messageBus.Publish(msg); err != nil {
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	stats := messageBus.GetStats()
	log.Printf("✅ Published %d messages", testMessages)
	log.Printf("📊 Bus stats: processed=%d, queued=%d",
		stats.TotalMessages,
		stats.QueueStats["normal-priority"].Size)

	log.Printf("🎉 Phase 1 Implementation Test: PASSED")
}

// BenchmarkMessageThroughput benchmarks message processing throughput
func BenchmarkMessageThroughput(b *testing.B) {
	processor := queue.NewBenchmarkMessageProcessor()

	busConfig := queue.MessageBusConfig{
		HighPriorityQueueSize:   65536,
		NormalPriorityQueueSize: 65536,
		LowPriorityQueueSize:    65536,
		DropPolicy:              queue.DropOldest,
		WorkerPoolConfig: queue.WorkerPoolConfig{
			WorkerCount:      8,
			MessageProcessor: processor,
			BatchSize:        100,
			BatchTimeout:     1 * time.Millisecond,
			EnableProfiling:  false,
		},
		EnableTopicQueues: true,
	}

	messageBus := queue.NewMessageBus(busConfig)
	messageBus.Start()
	defer messageBus.Stop()

	// Create test message
	testMessage := &types.PortaskMessage{
		ID:        "bench-message",
		Topic:     "benchmark",
		Timestamp: time.Now().Unix(),
		Payload:   []byte("Benchmark message payload for throughput testing"),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			messageBus.Publish(testMessage)
		}
	})

	// Wait for processing to complete
	time.Sleep(100 * time.Millisecond)

	processedCount, totalBytes, duration := processor.GetStats()
	messagesPerSecond := float64(processedCount) / duration.Seconds()
	mbPerSecond := float64(totalBytes) / 1024 / 1024 / duration.Seconds()

	b.ReportMetric(messagesPerSecond, "msg/sec")
	b.ReportMetric(mbPerSecond, "MB/sec")

	log.Printf("🚀 Benchmark Results:")
	log.Printf("   Messages processed: %d", processedCount)
	log.Printf("   Duration: %v", duration)
	log.Printf("   Throughput: %.0f msg/sec", messagesPerSecond)
	log.Printf("   Bandwidth: %.2f MB/sec", mbPerSecond)
}

func main() {
	// Run the test
	t := &testing.T{}
	TestPortaskPhase1(t)

	if t.Failed() {
		log.Fatal("Tests failed!")
	}

	log.Printf("✅ All tests passed!")
}
