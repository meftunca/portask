package benchmarks

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/memory"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// TestPhase2AllocationImprovements tests allocation elimination optimizations
func TestPhase2AllocationImprovements(t *testing.T) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-phase2",
		EnableCompression: false,
	}
	
	ctx := context.Background()
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
	
	t.Run("Allocation_Comparison", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		payload := make([]byte, 1024)
		
		// Measure allocations
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		messageCount := 1000
		messages := make([]*types.PortaskMessage, messageCount)
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%10), 0, nil, payload)
			messages[i] = msg
		}
		
		runtime.ReadMemStats(&m2)
		
		allocPerMsg := (m2.TotalAlloc - m1.TotalAlloc) / uint64(messageCount)
		mallocsPerMsg := float64(m2.Mallocs-m1.Mallocs) / float64(messageCount)
		
		// Return messages to pool
		for _, msg := range messages {
			memory.PutMessage(msg)
		}
		
		t.Logf("📊 Phase 2: Allocation Elimination Results")
		t.Logf("")
		t.Logf("Per Message:")
		t.Logf("  Bytes:   %d bytes/msg", allocPerMsg)
		t.Logf("  Mallocs: %.1f mallocs/msg", mallocsPerMsg)
		t.Logf("")
		t.Logf("📈 Progress:")
		t.Logf("  Baseline:  1,764 bytes/msg, 7.0 mallocs/msg")
		t.Logf("  Phase 1:   2,038 bytes/msg, 17.4 mallocs/msg (worse!)")
		t.Logf("  Phase 2:   %d bytes/msg, %.1f mallocs/msg", allocPerMsg, mallocsPerMsg)
		t.Logf("")
		
		if allocPerMsg < 1500 {
			improvement := (1764 - float64(allocPerMsg)) / 1764 * 100
			t.Logf("  ✅ %.1f%% allocation reduction from baseline!", improvement)
		}
		
		if mallocsPerMsg < 6 {
			t.Logf("  ✅ Malloc count reduced below baseline!")
		}
	})
	
	t.Run("Performance_WithOptimizations", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.HighThroughputConfig()
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		messageCount := 50000
		payload := make([]byte, 1024)
		
		t.Logf("🚀 Testing with Phase 2 optimizations (%d messages)...", messageCount)
		start := time.Now()
		
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		
		parallelWriter.Stop()
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		
		t.Logf("")
		t.Logf("✅ Results:")
		t.Logf("  Messages:    %d", messageCount)
		t.Logf("  Duration:    %v", duration)
		t.Logf("  Throughput:  %.0f msgs/sec", throughput)
		t.Logf("")
		t.Logf("📈 Progress:")
		t.Logf("  Baseline:     362,000 msgs/sec")
		t.Logf("  Phase 1:      152,555 msgs/sec (worse!)")
		t.Logf("  Phase 2:      %.0f msgs/sec", throughput)
		t.Logf("")
		
		if throughput > 362000 {
			improvement := (throughput - 362000) / 362000 * 100
			t.Logf("  🎉 +%.1f%% improvement over baseline!", improvement)
			
			if throughput > 470000 {
				t.Logf("  🎯 Phase 1 target (470K) achieved!")
			}
			if throughput > 500000 {
				t.Logf("  🚀 Phase 2 target (500K) achieved!")
			}
		} else {
			deficit := (362000 - throughput) / 362000 * 100
			t.Logf("  ⚠️  -%.1f%% from baseline (still optimizing...)", deficit)
		}
	})
	
	t.Run("GC_Pressure_Improvement", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		payload := make([]byte, 1024)
		
		// Measure GC stats
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)
		
		messageCount := 10000
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%10), 0, nil, payload)
			// Simulate usage
			_ = msg
			// Return to pool
			memory.PutMessage(msg)
		}
		
		runtime.ReadMemStats(&m2)
		
		gcRuns := m2.NumGC - m1.NumGC
		pauseTime := m2.PauseTotalNs - m1.PauseTotalNs
		
		t.Logf("🔄 GC Impact (%d messages):", messageCount)
		t.Logf("  GC Runs:       %d", gcRuns)
		t.Logf("  Total Pause:   %.2f ms", float64(pauseTime)/1000000.0)
		
		if gcRuns == 0 {
			t.Logf("  ✅ No GC triggered - excellent!")
		} else {
			avgPause := float64(pauseTime) / float64(gcRuns) / 1000000.0
			t.Logf("  Avg Pause:     %.2f ms", avgPause)
			
			if avgPause < 2 {
				t.Logf("  ✅ Very low GC pause times!")
			} else if avgPause < 5 {
				t.Logf("  ✅ Low GC pause times!")
			}
		}
	})
	
	t.Run("Translation_Benchmark", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		payload := make([]byte, 1024)
		
		start := time.Now()
		iterations := 100000
		
		for i := 0; i < iterations; i++ {
			msg, _ := translator.TranslateProduce("test-topic", 0, nil, payload)
			memory.PutMessage(msg)
		}
		
		duration := time.Since(start)
		rate := float64(iterations) / duration.Seconds()
		
		t.Logf("⚡ Translation Performance:")
		t.Logf("  Iterations:     %d", iterations)
		t.Logf("  Duration:       %v", duration)
		t.Logf("  Rate:           %.0f translations/sec", rate)
		t.Logf("  Avg Latency:    %.2f μs", duration.Seconds()/float64(iterations)*1000000)
		
		if rate > 10000000 {
			t.Logf("  ✅ > 10M translations/sec - excellent!")
		} else if rate > 5000000 {
			t.Logf("  ✅ > 5M translations/sec - good!")
		}
	})
}

// BenchmarkPhase2Optimizations benchmarks Phase 2 improvements
func BenchmarkPhase2Optimizations(b *testing.B) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-phase2-bench",
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
	
	b.Run("EndToEnd", func(b *testing.B) {
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.HighThroughputConfig()
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		payload := make([]byte, 1024)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%10), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		
		b.StopTimer()
		parallelWriter.Stop()
		
		b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
	})
	
	b.Run("TranslationOnly", func(b *testing.B) {
		translator := kafka.NewKafkaTranslator()
		payload := make([]byte, 1024)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			msg, _ := translator.TranslateProduce("test-topic", 0, nil, payload)
			memory.PutMessage(msg)
		}
		
		b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "translations/sec")
	})
}

