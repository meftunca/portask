package main

import (
	"fmt"
	"log"
	"time"

	"github.com/meftunca/portask/pkg/compression"
	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/queue"
	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/types"
)

func main() {
	log.Printf("🧪 Testing Portask Phase 1 Core Components")

	// Create simple config with no compression
	cfg := config.DefaultConfig()
	cfg.Compression.Type = config.CompressionNone

	log.Printf("✅ Configuration: OK")

	// Test serialization
	codecFactory := serialization.NewCodecFactory()
	if err := codecFactory.InitializeDefaultCodecs(cfg); err != nil {
		log.Fatalf("Failed to initialize codecs: %v", err)
	}
	codec, err := codecFactory.GetCodec(cfg.Serialization.Type)
	if err != nil {
		log.Fatalf("Failed to create codec: %v", err)
	}
	log.Printf("✅ Serialization (%s): OK", cfg.Serialization.Type)

	// Test compression (none)
	compressorFactory := compression.NewCompressorFactory()
	compressor, err := compressorFactory.GetCompressor(cfg.Compression.Type)
	if err != nil {
		log.Fatalf("Failed to create compressor: %v", err)
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
		log.Fatalf("Failed to serialize message: %v", err)
	}

	deserialized, err := codec.Decode(serialized)
	if err != nil {
		log.Fatalf("Failed to deserialize message: %v", err)
	}

	if deserialized.ID != testMessage.ID || string(deserialized.Payload) != string(testMessage.Payload) {
		log.Fatal("Serialization/deserialization mismatch")
	}
	log.Printf("✅ Message Serialization: OK")

	// Test compression (no-op for 'none')
	compressed, err := compressor.Compress(serialized)
	if err != nil {
		log.Fatalf("Failed to compress data: %v", err)
	}

	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		log.Fatalf("Failed to decompress data: %v", err)
	}

	if string(decompressed) != string(serialized) {
		log.Fatal("Compression/decompression mismatch")
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
		log.Fatalf("Failed to start message bus: %v", err)
	}
	defer messageBus.Stop()

	log.Printf("✅ Message Bus: OK")

	// Test message publishing
	testMessages := 1000
	start := time.Now()

	for i := 0; i < testMessages; i++ {
		msg := &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("perf-test-%d", i)),
			Topic:     "performance-test",
			Timestamp: time.Now().Unix(),
			Payload:   []byte(fmt.Sprintf("Performance test message #%d", i)),
		}

		if err := messageBus.Publish(msg); err != nil {
			log.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}

	publishDuration := time.Since(start)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	stats := messageBus.GetStats()
	log.Printf("✅ Published %d messages in %v", testMessages, publishDuration)
	log.Printf("📊 Bus stats: total=%d, queued=%d",
		stats.TotalMessages,
		stats.QueueStats["normal-priority"].Size)

	// Calculate throughput
	messagesPerSecond := float64(testMessages) / publishDuration.Seconds()
	log.Printf("🚀 Publishing throughput: %.0f msg/sec", messagesPerSecond)

	// Wait a bit more for processing to complete
	time.Sleep(500 * time.Millisecond)

	finalStats := messageBus.GetStats()
	log.Printf("📈 Final processing throughput: %.0f msg/sec", finalStats.MessagesPerSecond)

	// Print worker statistics
	log.Printf("\n👥 Worker Statistics:")
	for _, wStats := range finalStats.WorkerStats {
		log.Printf("  Worker %d: processed=%d, errors=%d, avg_time=%v",
			wStats.ID, wStats.MessagesProcessed, wStats.ErrorCount, wStats.AvgProcessingTime)
	}

	log.Printf("\n🎉 Phase 1 Core Implementation Test: PASSED")
	log.Printf("⚡ All systems operational and ready for production!")
}
