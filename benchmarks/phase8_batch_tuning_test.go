package benchmarks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/memory"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
)

// TestPhase8BatchTuning tests optimal batch size
func TestPhase8BatchTuning(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000 // Test size

	// Test various batch sizes
	batchSizes := []int{50, 100, 200, 500, 1000, 2000}
	results := make(map[int]float64)

	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-phase8",
		EnableCompression: false,
	}

	dragonflyStore, err := dragonfly.NewDragonflyStore(dfConfig)
	if err != nil {
		t.Skipf("Dragonfly not available: %v", err)
		return
	}

	if err := dragonflyStore.Connect(ctx); err != nil {
		t.Skipf("Connection failed: %v", err)
		return
	}
	defer dragonflyStore.Close()

	t.Logf("")
	t.Logf("🔬 Phase 8: Batch Size Tuning")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Testing %d messages with various batch sizes...", messageCount)
	t.Logf("")

	for _, batchSize := range batchSizes {
		batchSize := batchSize

		t.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(t *testing.T) {
			dragonflyStore.GetClient().FlushDB(ctx)
			time.Sleep(100 * time.Millisecond)

			kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
			storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}

			// Custom config with specific batch size
			config := &processor.ParallelBatchWriterConfig{
				NumShards:     32,
				FlushInterval: 10 * time.Millisecond,
				BatchSize:     batchSize,
				MaxRetries:    3,
			}

			asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
			asyncWriter.Start(ctx)

			start := time.Now()
			for i := 0; i < messageCount; i++ {
				msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
				asyncWriter.Write(msg)
				memory.PutMessage(msg)
			}

			time.Sleep(200 * time.Millisecond) // Wait for async
			asyncWriter.Stop()

			duration := time.Since(start)
			throughput := float64(messageCount) / duration.Seconds()
			results[batchSize] = throughput

			metrics := asyncWriter.GetMetrics()
			avgBatch := float64(messageCount) / float64(metrics.TotalBatchesWritten.Load())

			t.Logf("Batch Size: %4d | Throughput: %6.0f msgs/sec | Batches: %d | Avg: %.0f msgs/batch",
				batchSize, throughput, metrics.TotalBatchesWritten.Load(), avgBatch)
		})

		time.Sleep(200 * time.Millisecond) // Cool down between tests
	}

	t.Run("Summary", func(t *testing.T) {
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("📊 Batch Size Tuning Results")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")

		// Find optimal batch size
		var optimalSize int
		var optimalThroughput float64

		t.Logf("Results Summary:")
		for _, size := range batchSizes {
			throughput := results[size]
			marker := "  "
			if throughput > optimalThroughput {
				optimalThroughput = throughput
				optimalSize = size
				marker = "🎯"
			}
			t.Logf("  %s Batch %4d: %6.0f msgs/sec", marker, size, throughput)
		}

		t.Logf("")
		t.Logf("🏆 Optimal Configuration:")
		t.Logf("  Batch Size:    %d messages", optimalSize)
		t.Logf("  Throughput:    %.0f msgs/sec", optimalThroughput)

		// Calculate improvement over baseline (100)
		baseline := results[100]
		if baseline > 0 {
			improvement := (optimalThroughput - baseline) / baseline * 100
			t.Logf("  vs Baseline:   +%.1f%%", improvement)
		}

		t.Logf("")
		t.Logf("💡 Analysis:")

		// Analyze trend
		if results[50] < results[100] && results[100] < results[200] {
			t.Logf("  Trend: Throughput increases with batch size ✅")
			t.Logf("  Reason: Fewer round-trips = better performance")
		} else if results[1000] < results[500] {
			t.Logf("  Trend: Large batches show diminishing returns ⚠️")
			t.Logf("  Reason: Latency overhead starts to dominate")
		}

		t.Logf("")
		t.Logf("📝 Recommendation:")

		if optimalSize <= 100 {
			t.Logf("  Use SMALL batches (50-100)")
			t.Logf("  Best for: Low latency workloads")
		} else if optimalSize <= 500 {
			t.Logf("  Use MEDIUM batches (200-500)")
			t.Logf("  Best for: Balanced throughput/latency")
		} else {
			t.Logf("  Use LARGE batches (1000+)")
			t.Logf("  Best for: Maximum throughput, can tolerate latency")
		}

		t.Logf("")
		t.Logf("⚙️  Configuration Update:")
		t.Logf("  processor.HighThroughputConfig()")
		t.Logf("    BatchSize: %d → %d", 100, optimalSize)
		t.Logf("")
	})
}

// BenchmarkBatchSizes benchmarks different batch sizes
func BenchmarkBatchSizes(b *testing.B) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-batch-bench",
		EnableCompression: false,
	}

	ctx := context.Background()
	dragonflyStore, err := dragonfly.NewDragonflyStore(dfConfig)
	if err != nil {
		b.Skipf("Dragonfly not available: %v", err)
		return
	}

	if err := dragonflyStore.Connect(ctx); err != nil {
		b.Skipf("Connection failed: %v", err)
		return
	}
	defer dragonflyStore.Close()

	batchSizes := []int{100, 200, 500, 1000}

	for _, batchSize := range batchSizes {
		batchSize := batchSize

		b.Run(fmt.Sprintf("Batch_%d", batchSize), func(b *testing.B) {
			translator := kafka.NewKafkaTranslator()
			kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
			storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}

			config := &processor.ParallelBatchWriterConfig{
				NumShards:     32,
				FlushInterval: 10 * time.Millisecond,
				BatchSize:     batchSize,
				MaxRetries:    3,
			}

			asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
			asyncWriter.Start(ctx)
			defer asyncWriter.Stop()

			payload := make([]byte, 1024)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%10), 0, nil, payload)
				asyncWriter.Write(msg)
				memory.PutMessage(msg)
			}

			b.StopTimer()
			time.Sleep(50 * time.Millisecond)
			asyncWriter.Stop()

			b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
		})
	}
}
