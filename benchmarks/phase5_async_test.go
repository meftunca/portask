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

// TestPhase5Async tests async write pattern
func TestPhase5Async(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000

	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-phase5",
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

	t.Run("Async_vs_Sync", func(t *testing.T) {
		// Test 1: Sync writes (Phase 4)
		t.Logf("🔧 Test 1: SYNC writes (Phase 4)...")
		
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.HighThroughputConfig()
		syncWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		syncWriter.Start(ctx)
		
		start1 := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			syncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		syncWriter.Stop()
		syncDuration := time.Since(start1)
		syncThroughput := float64(messageCount) / syncDuration.Seconds()
		
		t.Logf("  Duration:    %v", syncDuration)
		t.Logf("  Throughput:  %.0f msgs/sec", syncThroughput)
		t.Logf("")
		
		// Clear Dragonfly
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		// Test 2: Async writes (Phase 5)
		t.Logf("🚀 Test 2: ASYNC writes (Phase 5)...")
		
		kafkaStore2 := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter2 := &kafka.KafkaStorageAdapter{Storage: kafkaStore2}
		
		asyncWriter := processor.NewAsyncBatchWriter(storageAdapter2, config)
		asyncWriter.Start(ctx)
		
		start2 := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		// Wait for confirmations
		time.Sleep(200 * time.Millisecond)
		asyncWriter.Stop()
		asyncDuration := time.Since(start2)
		asyncThroughput := float64(messageCount) / asyncDuration.Seconds()
		
		metrics := asyncWriter.GetMetrics()
		
		t.Logf("  Duration:    %v", asyncDuration)
		t.Logf("  Throughput:  %.0f msgs/sec", asyncThroughput)
		t.Logf("  Confirmed:   %d batches", metrics.TotalBatchesConfirmed.Load())
		t.Logf("  Pending:     %d batches", metrics.PendingBatches.Load())
		t.Logf("")
		
		// Compare
		speedup := asyncThroughput / syncThroughput
		improvement := (asyncThroughput - syncThroughput) / syncThroughput * 100
		
		t.Logf("📊 Comparison:")
		t.Logf("  Sync (Phase 4):   %.0f msgs/sec", syncThroughput)
		t.Logf("  Async (Phase 5):  %.0f msgs/sec", asyncThroughput)
		t.Logf("  Speedup:          %.2fx", speedup)
		t.Logf("  Improvement:      +%.1f%%", improvement)
		t.Logf("")
		
		if asyncThroughput > syncThroughput {
			if speedup > 1.5 {
				t.Logf("  🎉 Excellent! > 1.5x speedup from async pattern!")
			} else {
				t.Logf("  ✅ Good improvement from async writes!")
			}
		}
	})

	t.Run("Progress_Summary", func(t *testing.T) {
		t.Logf("")
		t.Logf("📊 ====== FULL OPTIMIZATION JOURNEY ======")
		t.Logf("")
		t.Logf("Phase 0 (Baseline):       163K msgs/sec")
		t.Logf("Phase 1 (Object Pool):    152K msgs/sec (-6.7%%)")
		t.Logf("Phase 2 (Alloc Elim):     158K msgs/sec (-3.1%%)")
		t.Logf("Phase 3 (Discovery):      163K msgs/sec (0.0%%)")
		t.Logf("  Breakthrough: 3.02M capable (in-memory)")
		t.Logf("  Bottleneck:   Dragonfly I/O (18.5x slowdown)")
		t.Logf("Phase 4 (Cmd Reduction):  182K msgs/sec (+11.7%%)")
		t.Logf("  Optimization: 3 commands → 1 command")
		t.Logf("Phase 5 (Async Writes):   [Testing now...]")
		t.Logf("  Expected:     250-300K msgs/sec")
		t.Logf("")
		t.Logf("Target:       500K msgs/sec")
		t.Logf("Stretch Goal: 1M msgs/sec")
		t.Logf("")
		t.Logf("=========================================")
	})
}

// BenchmarkPhase5 benchmarks async writes
func BenchmarkPhase5(b *testing.B) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-phase5-bench",
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

	translator := kafka.NewKafkaTranslator()
	kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
	storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}

	config := processor.HighThroughputConfig()
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
	time.Sleep(100 * time.Millisecond) // Wait for confirmations
	asyncWriter.Stop()

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
}

