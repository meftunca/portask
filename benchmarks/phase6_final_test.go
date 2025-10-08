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

// TestPhase6Final tests final optimized configuration
func TestPhase6Final(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 100000 // Larger test for final validation

	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-phase6",
		EnableCompression: false,
	}

	dragonflyStore, err := dragonfly.NewDragonflyStore(dfConfig)
	if err != nil {
		t.Skipf("Dragonfly not available: %v", err)
		return
	}

	if err := dragonflyStore.Connect(ctx); err != nil {
		t.Skipf("Dragonfly connection failed: %v", err)
		return
	}
	defer dragonflyStore.Close()

	dragonflyStore.GetClient().FlushDB(ctx)

	t.Run("Final_Optimized_Configuration", func(t *testing.T) {
		t.Logf("🚀 Phase 6: Final Optimized Configuration")
		t.Logf("   Testing with %d messages...", messageCount)
		t.Logf("")
		
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		// Use high throughput config (already optimal)
		config := processor.HighThroughputConfig()
		t.Logf("Configuration:")
		t.Logf("  Shards:        %d", config.NumShards)
		t.Logf("  Batch Size:    %d", config.BatchSize)
		t.Logf("  Flush Interval: %v", config.FlushInterval)
		t.Logf("  Max Retries:   %d", config.MaxRetries)
		t.Logf("")
		
		asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
		asyncWriter.Start(ctx)
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		// Allow time for async confirmations
		time.Sleep(500 * time.Millisecond)
		asyncWriter.Stop()
		
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		dataRate := float64(messageCount*1024) / duration.Seconds() / 1024 / 1024
		
		metrics := asyncWriter.GetMetrics()
		
		t.Logf("✅ Results:")
		t.Logf("  Messages:      %d", messageCount)
		t.Logf("  Duration:      %v", duration)
		t.Logf("  Throughput:    %.0f msgs/sec 🚀", throughput)
		t.Logf("  Data Rate:     %.2f MB/s", dataRate)
		t.Logf("  Avg Latency:   %.2f μs", duration.Seconds()/float64(messageCount)*1000000)
		t.Logf("")
		t.Logf("📊 Async Metrics:")
		t.Logf("  Total Batches: %d", metrics.TotalBatchesWritten.Load())
		t.Logf("  Confirmed:     %d", metrics.TotalBatchesConfirmed.Load())
		t.Logf("  Avg Batch:     %.0f msgs", float64(messageCount)/float64(metrics.TotalBatchesWritten.Load()))
		t.Logf("  Failed:        %d", metrics.FailedBatches.Load())
		t.Logf("")
		
		// Compare with baseline
		baseline := 163000.0
		improvement := (throughput - baseline) / baseline * 100
		
		t.Logf("📈 Progress vs Baseline:")
		t.Logf("  Baseline:      163K msgs/sec")
		t.Logf("  Current:       %.0f msgs/sec", throughput)
		t.Logf("  Improvement:   +%.1f%%", improvement)
		t.Logf("")
		
		// Goal tracking
		if throughput > 500000 {
			t.Logf("  🎉 TARGET ACHIEVED! > 500K msgs/sec!")
		} else if throughput > 300000 {
			t.Logf("  🎯 Excellent progress! > 300K msgs/sec!")
		} else if throughput > 250000 {
			t.Logf("  ✅ Good progress! > 250K msgs/sec!")
		} else if throughput > 200000 {
			t.Logf("  ✅ Phase 6 target achieved! > 200K msgs/sec!")
		}
		
		// Calculate theoretical max based on in-memory test
		theoreticalMax := 3020000.0
		efficiency := throughput / theoreticalMax * 100
		
		t.Logf("")
		t.Logf("🔬 Efficiency Analysis:")
		t.Logf("  Theoretical Max: %.2fM msgs/sec (in-memory)", theoreticalMax/1000000)
		t.Logf("  Current:         %.2fM msgs/sec (Dragonfly)", throughput/1000000)
		t.Logf("  Efficiency:      %.1f%% of theoretical max", efficiency)
		t.Logf("  Storage Overhead: %.1fx slowdown", theoreticalMax/throughput)
	})

	t.Run("Complete_Journey_Summary", func(t *testing.T) {
		t.Logf("")
		t.Logf("=" + string(make([]byte, 60)) + "=")
		for i := range string(make([]byte, 61)) {
			_ = i
		}
		t.Logf("🎯 COMPLETE OPTIMIZATION JOURNEY")
		t.Logf("=" + string(make([]byte, 60)) + "=")
		t.Logf("")
		t.Logf("Phase 0: Baseline")
		t.Logf("  Throughput: 163,000 msgs/sec")
		t.Logf("  Status: 🔴 Starting point")
		t.Logf("")
		t.Logf("Phase 1: Object Pooling")
		t.Logf("  Throughput: 152,000 msgs/sec (-6.7%%)")
		t.Logf("  Status: ❌ Overhead > benefit")
		t.Logf("  Lesson: Pooling alone insufficient")
		t.Logf("")
		t.Logf("Phase 2: Allocation Elimination")
		t.Logf("  Throughput: 158,000 msgs/sec (-3.1%%)")
		t.Logf("  Status: ❌ Still below baseline")
		t.Logf("  Lesson: Translator not the bottleneck")
		t.Logf("")
		t.Logf("Phase 3: Bottleneck Discovery")
		t.Logf("  In-Memory: 3,020,000 msgs/sec")
		t.Logf("  Dragonfly: 163,000 msgs/sec")
		t.Logf("  Status: ✅ BREAKTHROUGH!")
		t.Logf("  Discovery: Storage I/O is 18.5x slower")
		t.Logf("  Lesson: Focus on storage optimization")
		t.Logf("")
		t.Logf("Phase 4: Command Reduction")
		t.Logf("  Optimization: 3 commands → 1 command")
		t.Logf("  Throughput: 182,000 msgs/sec (+11.7%%)")
		t.Logf("  Status: ✅ First real improvement!")
		t.Logf("")
		t.Logf("Phase 5: Async Writes")
		t.Logf("  Optimization: Fire-and-forget pattern")
		t.Logf("  Throughput: 199,000 msgs/sec (+22.1%%)")
		t.Logf("  Status: ✅ Significant boost!")
		t.Logf("")
		t.Logf("Phase 6: Final Optimization")
		t.Logf("  Status: [Testing now...]")
		t.Logf("  Connection Pool: 1000 connections (already optimal)")
		t.Logf("  Async Pattern: Enabled")
		t.Logf("  Command Reduction: Enabled")
		t.Logf("")
		t.Logf("=" + string(make([]byte, 60)) + "=")
		t.Logf("")
		t.Logf("🎓 Key Learnings:")
		t.Logf("  1. Profile before optimizing")
		t.Logf("  2. Find the real bottleneck")
		t.Logf("  3. Application layer is fast (3M capable)")
		t.Logf("  4. Storage I/O is the limiting factor")
		t.Logf("  5. Multiple small optimizations compound")
		t.Logf("")
		t.Logf("🚀 Next Steps for 500K+:")
		t.Logf("  • Compression (CPU vs Network trade-off)")
		t.Logf("  • Batch size tuning")
		t.Logf("  • Alternative storage (RocksDB, BadgerDB)")
		t.Logf("  • Horizontal scaling (multiple Dragonfly)")
		t.Logf("")
	})
}

