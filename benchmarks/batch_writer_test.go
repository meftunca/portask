package benchmarks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
)

// TestBatchWriterWithDragonfly tests the batch writer with real Dragonfly
func TestBatchWriterWithDragonfly(t *testing.T) {
	// Setup Dragonfly
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-batch-test",
		EnableCompression: false,
	}

	ctx := context.Background()
	dragonflyStore, err := dragonfly.NewDragonflyStore(dfConfig)
	if err != nil {
		t.Fatalf("Failed to create Dragonfly store: %v", err)
	}

	err = dragonflyStore.Connect(ctx)
	if err != nil {
		t.Skipf("Dragonfly not available: %v. Please ensure Dragonfly is running on localhost:6379", err)
		return
	}
	defer dragonflyStore.Close()

	// Clear test data
	dragonflyStore.GetClient().FlushDB(ctx)

	t.Log("✅ Connected to Dragonfly")

	t.Run("BatchWrite_vs_DirectWrite", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()

		// Test 1: Direct write (old way)
		t.Log("🔧 Test 1: Direct Write (no batching)")
		directStartTime := time.Now()
		directCount := 0

		for i := 0; i < 100; i++ {
			msg, _ := translator.TranslateProduce("direct-topic", 0, nil, []byte(fmt.Sprintf("message-%d", i)))
			err := dragonflyStore.Store(ctx, msg)
			if err != nil {
				t.Errorf("Failed to store message %d: %v", i, err)
			}
			directCount++
		}

		directDuration := time.Since(directStartTime)
		directThroughput := float64(directCount) / directDuration.Seconds()

		t.Logf("  ✅ Direct Write: %d messages in %v", directCount, directDuration)
		t.Logf("     Throughput: %.0f msgs/sec", directThroughput)

		// Test 2: Batch write (new way with bridge)
		dragonflyStore.GetClient().FlushDB(ctx)

		t.Log("🔧 Test 2: Batch Write (10ms or 1000 messages)")

		// Create processor with batch writing enabled
		proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
		proc.Start(ctx)
		defer proc.Stop()

		// Create kafka store adapter
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)

		// Create bridge (will create batch writer internally)
		bridge := kafka.NewProcessorBridge(proc, kafkaStore)
		defer bridge.Stop() // Important: flush remaining messages

		batchStartTime := time.Now()
		batchCount := 0

		for i := 0; i < 100; i++ {
			msg, _ := translator.TranslateProduce("batch-topic", 0, nil, []byte(fmt.Sprintf("message-%d", i)))
			_, err := bridge.ProduceMessage(ctx, msg)
			if err != nil {
				t.Errorf("Failed to produce message %d: %v", i, err)
			}
			batchCount++
		}

		// Wait for batch flush
		time.Sleep(20 * time.Millisecond)

		batchDuration := time.Since(batchStartTime)
		batchThroughput := float64(batchCount) / batchDuration.Seconds()

		t.Logf("  ✅ Batch Write: %d messages in %v", batchCount, batchDuration)
		t.Logf("     Throughput: %.0f msgs/sec", batchThroughput)

		// Compare
		improvement := (batchThroughput - directThroughput) / directThroughput * 100
		t.Logf("")
		t.Logf("📊 Comparison:")
		t.Logf("   Direct Write:  %.0f msgs/sec", directThroughput)
		t.Logf("   Batch Write:   %.0f msgs/sec", batchThroughput)
		t.Logf("   Improvement:   %.1f%% 🚀", improvement)
	})

	t.Run("HighThroughputBatchWrite", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)

		translator := kafka.NewKafkaTranslator()
		proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
		proc.Start(ctx)
		defer proc.Stop()

		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		bridge := kafka.NewProcessorBridge(proc, kafkaStore)
		defer bridge.Stop()

		messageCount := 10000
		payload := make([]byte, 1024) // 1KB payload

		t.Logf("🚀 Sending %d messages (1KB each)...", messageCount)
		start := time.Now()

		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce("high-throughput", 0, nil, payload)
			bridge.ProduceMessage(ctx, msg)
		}

		// Wait for all batches to flush
		time.Sleep(50 * time.Millisecond)
		bridge.Stop() // Force final flush

		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		dataRate := float64(messageCount*1024) / duration.Seconds() / 1024 / 1024 // MB/s

		t.Logf("✅ Completed in %v", duration)
		t.Logf("📊 Results:")
		t.Logf("   Messages:    %d", messageCount)
		t.Logf("   Duration:    %v", duration)
		t.Logf("   Throughput:  %.0f msgs/sec", throughput)
		t.Logf("   Data Rate:   %.2f MB/s", dataRate)
		t.Logf("   Avg Latency: %.2f ms", float64(duration.Milliseconds())/float64(messageCount))
	})
}

// BenchmarkBatchWriter benchmarks batch writer performance
func BenchmarkBatchWriter(b *testing.B) {
	// Setup Dragonfly
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-bench-batch",
		EnableCompression: false,
	}

	ctx := context.Background()
	dragonflyStore, err := dragonfly.NewDragonflyStore(dfConfig)
	if err != nil {
		b.Skipf("Dragonfly not available: %v", err)
		return
	}

	if err := dragonflyStore.Connect(ctx); err != nil {
		b.Skipf("Dragonfly connection failed: %v", err)
		return
	}
	defer dragonflyStore.Close()

	dragonflyStore.GetClient().FlushDB(ctx)

	// Setup
	translator := kafka.NewKafkaTranslator()
	proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
	proc.Start(ctx)
	defer proc.Stop()

	kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
	bridge := kafka.NewProcessorBridge(proc, kafkaStore)
	defer bridge.Stop()

	payload := make([]byte, 1024) // 1KB

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg, _ := translator.TranslateProduce("bench-topic", 0, nil, payload)
		bridge.ProduceMessage(ctx, msg)
	}

	b.StopTimer()
	bridge.Stop() // Final flush

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
}
