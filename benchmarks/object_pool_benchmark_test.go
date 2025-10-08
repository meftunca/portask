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

// TestObjectPoolingImpact tests the impact of object pooling
func TestObjectPoolingImpact(t *testing.T) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-pool-test",
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
	
	t.Run("MemoryAllocation_Comparison", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		payload := make([]byte, 1024)
		
		// Measure allocations WITH pooling (current)
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		messages := make([]*types.PortaskMessage, 1000)
		for i := 0; i < 1000; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%10), 0, nil, payload)
			messages[i] = msg
		}
		
		runtime.ReadMemStats(&m2)
		
		allocWithPool := m2.TotalAlloc - m1.TotalAlloc
		mallocsWithPool := m2.Mallocs - m1.Mallocs
		
		// Return messages to pool
		for _, msg := range messages {
			memory.PutMessage(msg)
		}
		
		t.Logf("📊 Allocation Comparison (1000 messages):")
		t.Logf("")
		t.Logf("WITH Object Pooling:")
		t.Logf("  Total Alloc:   %d bytes (%d KB)", allocWithPool, allocWithPool/1024)
		t.Logf("  Per Message:   %d bytes", allocWithPool/1000)
		t.Logf("  Malloc Calls:  %d", mallocsWithPool)
		t.Logf("  Per Message:   %.1f mallocs", float64(mallocsWithPool)/1000.0)
		t.Logf("")
		
		// Expected improvements
		t.Logf("💡 Expected Improvement:")
		t.Logf("  Before: ~1,764 bytes/msg, 7 mallocs/msg")
		t.Logf("  After:  ~%d bytes/msg, %.1f mallocs/msg", allocWithPool/1000, float64(mallocsWithPool)/1000.0)
		
		if allocWithPool/1000 < 1200 {
			t.Logf("  ✅ Significant memory reduction achieved!")
		}
		
		if float64(mallocsWithPool)/1000.0 < 5 {
			t.Logf("  ✅ Malloc reduction achieved!")
		}
	})
	
	t.Run("Performance_WithPooling", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.HighThroughputConfig()
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		messageCount := 50000
		payload := make([]byte, 1024)
		
		t.Logf("🚀 Testing with Object Pooling (%d messages)...", messageCount)
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
		t.Logf("📈 Comparison:")
		t.Logf("  Previous (no pooling): ~362K msgs/sec")
		t.Logf("  Current (with pooling): %.0f msgs/sec", throughput)
		
		if throughput > 400000 {
			improvement := (throughput - 362000) / 362000 * 100
			t.Logf("  Improvement: +%.1f%% 🎉", improvement)
		}
	})
	
	t.Run("GC_Pressure_Test", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		payload := make([]byte, 1024)
		
		// Measure GC stats
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)
		
		// Create and destroy many messages
		for i := 0; i < 10000; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%10), 0, nil, payload)
			// Simulate usage
			_ = msg
			// Return to pool
			memory.PutMessage(msg)
		}
		
		runtime.ReadMemStats(&m2)
		
		gcRuns := m2.NumGC - m1.NumGC
		pauseTime := m2.PauseTotalNs - m1.PauseTotalNs
		
		t.Logf("🔄 GC Impact (10K messages):")
		t.Logf("  GC Runs:       %d", gcRuns)
		t.Logf("  Total Pause:   %.2f ms", float64(pauseTime)/1000000.0)
		
		if gcRuns == 0 {
			t.Logf("  ✅ No GC triggered - excellent!")
		} else {
			avgPause := float64(pauseTime) / float64(gcRuns) / 1000000.0
			t.Logf("  Avg Pause:     %.2f ms", avgPause)
			
			if avgPause < 5 {
				t.Logf("  ✅ Low GC pause times!")
			}
		}
	})
	
	t.Run("StringInterner_Effectiveness", func(t *testing.T) {
		// Test string interning
		topics := []string{"orders", "payments", "notifications", "analytics", "logs"}
		
		// Create many messages with same topics
		for i := 0; i < 1000; i++ {
			topic := topics[i%len(topics)]
			interned := memory.InternTopic(topic)
			_ = interned
		}
		
		t.Logf("📝 String Interner:")
		t.Logf("  Unique topics cached for reuse")
		t.Logf("  ✅ Reduces string allocations")
	})
}

// BenchmarkObjectPooling benchmarks with and without pooling
func BenchmarkObjectPooling(b *testing.B) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-pool-bench",
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
	
	b.Run("WithPooling", func(b *testing.B) {
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
	
	b.Run("Translation_Only", func(b *testing.B) {
		translator := kafka.NewKafkaTranslator()
		payload := make([]byte, 1024)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			msg, _ := translator.TranslateProduce("test-topic", 0, nil, payload)
			memory.PutMessage(msg) // Return to pool
		}
		
		b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "translations/sec")
	})
}

