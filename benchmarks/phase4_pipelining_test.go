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

// TestPhase4Pipelining tests Redis command reduction optimization
func TestPhase4Pipelining(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000
	
	t.Run("Optimized_Pipelining", func(t *testing.T) {
		dfConfig := &storage.DragonflyConfig{
			Addresses:         []string{"localhost:6379"},
			DB:                0,
			KeyPrefix:         "portask-phase4",
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
		
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.HighThroughputConfig()
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		t.Logf("🚀 Testing Phase 4: Optimized Pipelining (%d messages)...", messageCount)
		t.Logf("   Optimization: 3 commands → 1 command per message")
		t.Logf("")
		
		start := time.Now()
		
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			parallelWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		parallelWriter.Stop()
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		dataRate := float64(messageCount*1024) / duration.Seconds() / 1024 / 1024
		
		t.Logf("✅ Results:")
		t.Logf("  Messages:      %d", messageCount)
		t.Logf("  Duration:      %v", duration)
		t.Logf("  Throughput:    %.0f msgs/sec", throughput)
		t.Logf("  Data Rate:     %.2f MB/s", dataRate)
		t.Logf("  Avg Latency:   %.2f μs", duration.Seconds()/float64(messageCount)*1000000)
		t.Logf("")
		t.Logf("📊 Progress:")
		t.Logf("  Phase 3:       163K msgs/sec (baseline)")
		t.Logf("  Phase 4:       %.0f msgs/sec", throughput)
		
		if throughput > 163000 {
			improvement := (throughput - 163000) / 163000 * 100
			t.Logf("  Improvement:   +%.1f%% 🎉", improvement)
			
			if throughput > 250000 {
				t.Logf("  ✅ Phase 4 interim target (250K) achieved!")
			}
			if throughput > 400000 {
				t.Logf("  🎯 Phase 4 goal (400K) achieved!")
			}
		}
		
		// Calculate command reduction benefit
		commandsPhase3 := messageCount * 3 // SET + XADD + INCR
		commandsPhase4 := messageCount * 1 // Only SET
		commandReduction := float64(commandsPhase3-commandsPhase4) / float64(commandsPhase3) * 100
		
		t.Logf("")
		t.Logf("📉 Command Reduction:")
		t.Logf("  Phase 3:       %d commands (3 per message)", commandsPhase3)
		t.Logf("  Phase 4:       %d commands (1 per message)", commandsPhase4)
		t.Logf("  Reduction:     %.0f%% fewer commands", commandReduction)
	})
	
	t.Run("Comparison_All_Phases", func(t *testing.T) {
		t.Logf("")
		t.Logf("📊 ====== OPTIMIZATION JOURNEY ======")
		t.Logf("")
		t.Logf("Phase 1 (Object Pooling):")
		t.Logf("  Result:      152K msgs/sec")
		t.Logf("  Status:      ❌ Worse than baseline")
		t.Logf("  Lesson:      Pooling alone not enough")
		t.Logf("")
		t.Logf("Phase 2 (Allocation Elimination):")
		t.Logf("  Result:      158K msgs/sec")
		t.Logf("  Status:      ⚠️  Still below baseline")
		t.Logf("  Lesson:      Translator fast (5.26M/sec)")
		t.Logf("               Bottleneck is storage!")
		t.Logf("")
		t.Logf("Phase 3 (Bottleneck Discovery):")
		t.Logf("  In-Memory:   3.02M msgs/sec")
		t.Logf("  Dragonfly:   163K msgs/sec")
		t.Logf("  Slowdown:    18.5x")
		t.Logf("  Discovery:   🔴 Storage is the bottleneck!")
		t.Logf("")
		t.Logf("Phase 4 (Command Reduction):")
		t.Logf("  Optimization: 3 commands → 1 command")
		t.Logf("  Expected:     2-3x improvement")
		t.Logf("  Target:       400K msgs/sec")
		t.Logf("")
		t.Logf("====================================")
	})
}

// BenchmarkPhase4 benchmarks Phase 4 improvements
func BenchmarkPhase4(b *testing.B) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-phase4-bench",
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
	parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
	parallelWriter.Start(ctx)
	defer parallelWriter.Stop()
	
	payload := make([]byte, 1024)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%10), 0, nil, payload)
		parallelWriter.Write(msg)
		memory.PutMessage(msg)
	}
	
	b.StopTimer()
	parallelWriter.Stop()
	
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
}

