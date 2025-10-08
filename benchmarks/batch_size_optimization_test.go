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

// TestBatchSizeOptimization tests different batch sizes with parallel writes
func TestBatchSizeOptimization(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 100000 // 100K messages for better measurement

	dfConfig := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		DB:        0,
		KeyPrefix: "portask-batch-opt",
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
	t.Logf("🔬 BATCH SIZE OPTIMIZATION TEST")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Total Messages: %d", messageCount)
	t.Logf("Message Size: 1KB")
	t.Logf("SubBatchSize: 200 (fixed)")
	t.Logf("")

	results := make(map[int]struct {
		throughput   float64
		goroutines   int
		batchCount   int
		avgBatchFill float64
	})

	batchSizes := []int{500, 1000, 2000, 5000, 10000}

	for _, batchSize := range batchSizes {
		batchSize := batchSize

		t.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(t *testing.T) {
			dragonflyStore.GetClient().FlushDB(ctx)
			time.Sleep(100 * time.Millisecond)

			kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
			storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}

			config := processor.HighThroughputConfig()
			config.BatchSize = batchSize
			config.SubBatchSize = 200
			config.EnableParallelWrites = true

			// Calculate expected goroutines per batch
			expectedGoroutines := (batchSize + 200 - 1) / 200

			asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
			asyncWriter.Start(ctx)

			t.Logf("Testing BatchSize=%d (expect %d goroutines/batch)...", batchSize, expectedGoroutines)

			start := time.Now()
			for i := 0; i < messageCount; i++ {
				msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
				asyncWriter.Write(msg)
				memory.PutMessage(msg)
			}

			// Wait proportional to batch size
			waitTime := time.Duration(batchSize/100) * time.Millisecond
			if waitTime < 200*time.Millisecond {
				waitTime = 200 * time.Millisecond
			}
			time.Sleep(waitTime)

			asyncWriter.Stop()

			duration := time.Since(start)
			throughput := float64(messageCount) / duration.Seconds()

			metrics := asyncWriter.GetMetrics()
			batchCount := int(metrics.TotalBatchesWritten.Load())
			avgBatchFill := float64(messageCount) / float64(batchCount)

			results[batchSize] = struct {
				throughput   float64
				goroutines   int
				batchCount   int
				avgBatchFill float64
			}{
				throughput:   throughput,
				goroutines:   expectedGoroutines,
				batchCount:   batchCount,
				avgBatchFill: avgBatchFill,
			}

			t.Logf("  Throughput: %.0f msgs/sec", throughput)
			t.Logf("  Batches: %d", batchCount)
			t.Logf("  Avg Batch Fill: %.0f msgs", avgBatchFill)
			t.Logf("  Goroutines/Batch: %d", expectedGoroutines)
		})

		time.Sleep(500 * time.Millisecond)
	}

	// Summary
	t.Run("Summary", func(t *testing.T) {
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("📊 BATCH SIZE OPTIMIZATION RESULTS")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")

		baseline := results[500].throughput

		t.Logf("BatchSize | Goroutines | Batches | Avg Fill | Throughput    | vs 500")
		t.Logf("----------|------------|---------|----------|---------------|--------")

		for _, batchSize := range batchSizes {
			result := results[batchSize]
			improvement := ((result.throughput - baseline) / baseline) * 100

			marker := ""
			if improvement > 50 {
				marker = "🚀🚀"
			} else if improvement > 20 {
				marker = "🚀"
			} else if improvement > 0 {
				marker = "✅"
			} else {
				marker = ""
			}

			t.Logf("%5d     | %2d         | %4d    | %5.0f    | %6.0f msgs/s | %+5.0f%% %s",
				batchSize,
				result.goroutines,
				result.batchCount,
				result.avgBatchFill,
				result.throughput,
				improvement,
				marker)
		}

		t.Logf("")

		// Find optimal
		var optimalSize int
		var maxThroughput float64
		for size, result := range results {
			if result.throughput > maxThroughput {
				maxThroughput = result.throughput
				optimalSize = size
			}
		}

		optimalResult := results[optimalSize]
		improvement := ((maxThroughput - baseline) / baseline) * 100

		t.Logf("🏆 Optimal BatchSize: %d", optimalSize)
		t.Logf("   Goroutines/Batch: %d", optimalResult.goroutines)
		t.Logf("   Throughput: %.0f msgs/sec", maxThroughput)
		t.Logf("   Improvement: +%.0f%% vs BatchSize=500", improvement)
		t.Logf("")

		t.Logf("💡 Analysis:")
		t.Logf("")

		if optimalSize == 500 {
			t.Logf("  Smaller batches (500) are optimal")
			t.Logf("  Reason: Lower latency, faster flush cycles")
		} else if optimalSize >= 5000 {
			t.Logf("  Larger batches (%d) are optimal", optimalSize)
			t.Logf("  Reason: More parallel goroutines (%d), better throughput", optimalResult.goroutines)
			t.Logf("  Trade-off: Slightly higher latency per batch")
		} else {
			t.Logf("  Medium batches (%d) are optimal", optimalSize)
			t.Logf("  Reason: Good balance of parallelism and latency")
		}

		t.Logf("")
		t.Logf("📈 Parallelism Impact:")
		t.Logf("")

		result500 := results[500]
		result10k := results[10000]

		t.Logf("  BatchSize 500  → %d goroutines → %.0f msgs/sec",
			result500.goroutines, result500.throughput)
		t.Logf("  BatchSize 10K  → %d goroutines → %.0f msgs/sec",
			result10k.goroutines, result10k.throughput)

		goroutineRatio := float64(result10k.goroutines) / float64(result500.goroutines)
		throughputRatio := result10k.throughput / result500.throughput

		t.Logf("")
		t.Logf("  Goroutine increase: %.1fx", goroutineRatio)
		t.Logf("  Throughput increase: %.1fx", throughputRatio)

		if throughputRatio >= goroutineRatio*0.8 {
			t.Logf("  ✅ Parallelism scales well! (%.0f%% efficiency)", (throughputRatio/goroutineRatio)*100)
		} else {
			t.Logf("  ⚠️  Parallelism has diminishing returns")
		}

		t.Logf("")
	})
}
