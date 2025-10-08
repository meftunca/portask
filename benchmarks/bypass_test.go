package benchmarks

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/memory"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
)

// TestStorageBypass tests throughput with and without Dragonfly
// This identifies if storage is the bottleneck
func TestStorageBypass(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000
	
	t.Run("InMemory_MaxThroughput", func(t *testing.T) {
		// Test 1: In-memory storage (no I/O)
		memAdapter := NewInMemoryStorageAdapter()
		
		config := processor.HighThroughputConfig()
		parallelWriter := processor.NewParallelBatchWriter(memAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		t.Logf("🚀 Testing IN-MEMORY (no I/O) - %d messages...", messageCount)
		start := time.Now()
		
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			parallelWriter.Write(msg)
			memory.PutMessage(msg) // Return to pool
		}
		
		parallelWriter.Stop()
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		
		metrics := memAdapter.GetMetrics()
		
		t.Logf("")
		t.Logf("✅ IN-MEMORY Results:")
		t.Logf("  Messages:      %d", messageCount)
		t.Logf("  Duration:      %v", duration)
		t.Logf("  Throughput:    %.0f msgs/sec 🚀", throughput)
		t.Logf("  Avg Latency:   %.2f μs", duration.Seconds()/float64(messageCount)*1000000)
		t.Logf("  Write Ops:     %d", metrics["write_ops"])
		t.Logf("  Avg Batch:     %.0f msgs", float64(messageCount)/float64(metrics["write_ops"]))
		t.Logf("")
		
		if throughput > 1000000 {
			t.Logf("  🎉 > 1M msgs/sec achievable without storage I/O!")
		} else if throughput > 500000 {
			t.Logf("  ✅ > 500K msgs/sec - application logic is fast enough!")
		}
	})
	
	t.Run("Dragonfly_RealWorld", func(t *testing.T) {
		// Test 2: Dragonfly storage (real I/O)
		dfConfig := &storage.DragonflyConfig{
			Addresses:         []string{"localhost:6379"},
			DB:                0,
			KeyPrefix:         "portask-bypass",
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
		
		t.Logf("🚀 Testing DRAGONFLY (real I/O) - %d messages...", messageCount)
		start := time.Now()
		
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			parallelWriter.Write(msg)
			memory.PutMessage(msg) // Return to pool
		}
		
		parallelWriter.Stop()
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		
		t.Logf("")
		t.Logf("✅ DRAGONFLY Results:")
		t.Logf("  Messages:      %d", messageCount)
		t.Logf("  Duration:      %v", duration)
		t.Logf("  Throughput:    %.0f msgs/sec", throughput)
		t.Logf("  Avg Latency:   %.2f μs", duration.Seconds()/float64(messageCount)*1000000)
		t.Logf("")
	})
	
	t.Run("Comparison", func(t *testing.T) {
		// Run both tests in sequence for comparison
		memAdapter := NewInMemoryStorageAdapter()
		config := processor.HighThroughputConfig()
		
		// In-memory test
		parallelWriter1 := processor.NewParallelBatchWriter(memAdapter, config)
		parallelWriter1.Start(ctx)
		
		start1 := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			parallelWriter1.Write(msg)
			memory.PutMessage(msg)
		}
		parallelWriter1.Stop()
		memDuration := time.Since(start1)
		memThroughput := float64(messageCount) / memDuration.Seconds()
		
		// Dragonfly test
		dfConfig := &storage.DragonflyConfig{
			Addresses:         []string{"localhost:6379"},
			DB:                0,
			KeyPrefix:         "portask-bypass",
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
		
		parallelWriter2 := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter2.Start(ctx)
		
		start2 := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			parallelWriter2.Write(msg)
			memory.PutMessage(msg)
		}
		parallelWriter2.Stop()
		dfDuration := time.Since(start2)
		dfThroughput := float64(messageCount) / dfDuration.Seconds()
		
		// Compare
		slowdown := memThroughput / dfThroughput
		overhead := ((memDuration.Seconds() - dfDuration.Seconds()) / dfDuration.Seconds()) * 100
		
		t.Logf("")
		t.Logf("📊 ====== STORAGE BOTTLENECK ANALYSIS ======")
		t.Logf("")
		t.Logf("IN-MEMORY (No I/O):")
		t.Logf("  Throughput:  %.0f msgs/sec", memThroughput)
		t.Logf("  Duration:    %v", memDuration)
		t.Logf("")
		t.Logf("DRAGONFLY (Real I/O):")
		t.Logf("  Throughput:  %.0f msgs/sec", dfThroughput)
		t.Logf("  Duration:    %v", dfDuration)
		t.Logf("")
		t.Logf("IMPACT:")
		t.Logf("  Slowdown:    %.1fx", slowdown)
		t.Logf("  I/O Overhead: %.1f%%", overhead)
		t.Logf("")
		
		if slowdown > 5 {
			t.Logf("🔴 CONFIRMED: Storage I/O is the PRIMARY bottleneck!")
			t.Logf("   Application can handle %.0f msgs/sec", memThroughput)
			t.Logf("   Storage limits us to %.0f msgs/sec", dfThroughput)
			t.Logf("")
			t.Logf("💡 Optimization opportunities:")
			t.Logf("   1. Redis pipelining")
			t.Logf("   2. Async writes with confirmation")
			t.Logf("   3. Connection pooling")
			t.Logf("   4. Write buffer optimization")
			t.Logf("   5. Consider alternative storage (RocksDB, etc)")
		} else if slowdown > 2 {
			t.Logf("⚠️  Storage I/O is a significant bottleneck")
			t.Logf("   Optimization would provide %.1fx speedup", slowdown)
		} else {
			t.Logf("✅ Storage I/O is not the main bottleneck")
			t.Logf("   Look for optimization elsewhere")
		}
		t.Logf("")
		t.Logf("============================================")
	})
	
	t.Run("HighConcurrency_InMemory", func(t *testing.T) {
		// Stress test with in-memory to find max capacity
		memAdapter := NewInMemoryStorageAdapter()
		config := processor.HighThroughputConfig()
		parallelWriter := processor.NewParallelBatchWriter(memAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		testCount := 100000 // 100K messages
		numProducers := 8   // 8 concurrent producers
		
		t.Logf("🚀 Stress test: %d messages with %d producers...", testCount, numProducers)
		start := time.Now()
		
		var wg sync.WaitGroup
		for p := 0; p < numProducers; p++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < testCount/numProducers; i++ {
					msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
					parallelWriter.Write(msg)
					memory.PutMessage(msg)
				}
			}()
		}
		wg.Wait()
		
		parallelWriter.Stop()
		duration := time.Since(start)
		throughput := float64(testCount) / duration.Seconds()
		
		metrics := memAdapter.GetMetrics()
		
		t.Logf("")
		t.Logf("✅ Stress Test Results:")
		t.Logf("  Messages:      %d", testCount)
		t.Logf("  Producers:     %d", numProducers)
		t.Logf("  Duration:      %v", duration)
		t.Logf("  Throughput:    %.0f msgs/sec 🚀", throughput)
		t.Logf("  Data Rate:     %.2f MB/s", float64(testCount*1024)/duration.Seconds()/1024/1024)
		t.Logf("  Avg Latency:   %.2f μs", duration.Seconds()/float64(testCount)*1000000)
		t.Logf("  Write Ops:     %d", metrics["write_ops"])
		t.Logf("  Avg Batch:     %.0f msgs", float64(testCount)/float64(metrics["write_ops"]))
		t.Logf("")
		
		if throughput > 1000000 {
			t.Logf("  🎉 EXCELLENT: > 1M msgs/sec capacity!")
		} else if throughput > 500000 {
			t.Logf("  ✅ GOOD: > 500K msgs/sec capacity!")
		}
		
		t.Logf("")
		t.Logf("💡 This is the MAXIMUM throughput without storage I/O")
		t.Logf("   Any optimization below this is storage-limited")
	})
}

