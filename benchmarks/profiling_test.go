package benchmarks

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// TestProfilingBottlenecks identifies performance bottlenecks
func TestProfilingBottlenecks(t *testing.T) {
	// Setup Dragonfly
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-profile",
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
	
	t.Run("IdentifyBottlenecks", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
		proc.Start(ctx)
		defer proc.Stop()
		
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		bridge := kafka.NewProcessorBridge(proc, kafkaStore)
		defer bridge.Stop()
		
		messageCount := 10000
		payload := make([]byte, 1024) // 1KB
		
		// Start CPU profiling
		cpuFile, _ := os.Create("cpu.prof")
		pprof.StartCPUProfile(cpuFile)
		defer pprof.StopCPUProfile()
		
		// Measure each stage
		var (
			translateTime time.Duration
			processTime   time.Duration
			writeTime     time.Duration
		)
		
		start := time.Now()
		
		for i := 0; i < messageCount; i++ {
			// 1. Translation
			t1 := time.Now()
			msg, _ := translator.TranslateProduce("profile-topic", 0, nil, payload)
			translateTime += time.Since(t1)
			
			// 2. Processing
			t2 := time.Now()
			processedMsg, _ := proc.ProcessMessage(ctx, msg)
			processTime += time.Since(t2)
			
			// 3. Batch Write
			t3 := time.Now()
			bridge.ProduceMessage(ctx, processedMsg)
			writeTime += time.Since(t3)
		}
		
		totalTime := time.Since(start)
		bridge.Stop() // Final flush
		
		// Memory profiling
		memFile, _ := os.Create("mem.prof")
		runtime.GC()
		pprof.WriteHeapProfile(memFile)
		memFile.Close()
		
		// Report bottlenecks
		t.Logf("🔍 Bottleneck Analysis (%d messages):", messageCount)
		t.Logf("")
		t.Logf("Total Time:     %v", totalTime)
		t.Logf("Throughput:     %.0f msgs/sec", float64(messageCount)/totalTime.Seconds())
		t.Logf("")
		t.Logf("📊 Time Breakdown:")
		t.Logf("  Translation:  %v (%.1f%%)", translateTime, float64(translateTime)/float64(totalTime)*100)
		t.Logf("  Processing:   %v (%.1f%%)", processTime, float64(processTime)/float64(totalTime)*100)
		t.Logf("  Write:        %v (%.1f%%)", writeTime, float64(writeTime)/float64(totalTime)*100)
		t.Logf("")
		t.Logf("⚡ Per-Operation Latency:")
		t.Logf("  Translate:    %.2f μs", float64(translateTime.Microseconds())/float64(messageCount))
		t.Logf("  Process:      %.2f μs", float64(processTime.Microseconds())/float64(messageCount))
		t.Logf("  Write:        %.2f μs", float64(writeTime.Microseconds())/float64(messageCount))
		
		// Identify bottleneck
		maxTime := translateTime
		bottleneck := "Translation"
		if processTime > maxTime {
			maxTime = processTime
			bottleneck = "Processing"
		}
		if writeTime > maxTime {
			maxTime = writeTime
			bottleneck = "Write"
		}
		
		t.Logf("")
		t.Logf("🎯 BOTTLENECK: %s (%.1f%% of total time)", bottleneck, float64(maxTime)/float64(totalTime)*100)
		t.Logf("")
		t.Logf("💡 Optimization Opportunities:")
		
		if processTime > totalTime/4 {
			t.Logf("  • Reduce ProcessMessage overhead")
			t.Logf("  • Consider zero-copy processing")
			t.Logf("  • Optimize validation/compression")
		}
		
		if translateTime > totalTime/4 {
			t.Logf("  • Optimize translation logic")
			t.Logf("  • Reduce allocations in TranslateProduce")
		}
		
		if writeTime > totalTime/4 {
			t.Logf("  • Parallel batch writers")
			t.Logf("  • Larger batches")
		}
		
		t.Logf("")
		t.Logf("📁 Profiles written:")
		t.Logf("  cpu.prof - Run: go tool pprof -http=:8080 cpu.prof")
		t.Logf("  mem.prof - Run: go tool pprof -http=:8080 mem.prof")
	})
	
	t.Run("ProcessorOverhead", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		
		// Test 1: Direct write (no processor)
		t.Log("🔧 Test 1: Direct Write (bypassing processor)")
		start1 := time.Now()
		for i := 0; i < 1000; i++ {
			msg := &types.PortaskMessage{
				ID:        types.MessageID(fmt.Sprintf("%d", time.Now().UnixNano())),
				Topic:     types.TopicName("direct"),
				Payload:   make([]byte, 1024),
				Timestamp: time.Now().UnixNano(),
				TTL:       int64(time.Hour),
			}
			dragonflyStore.Store(ctx, msg)
		}
		directTime := time.Since(start1)
		directThroughput := float64(1000) / directTime.Seconds()
		
		t.Logf("  Throughput: %.0f msgs/sec", directThroughput)
		
		// Test 2: With processor
		dragonflyStore.GetClient().FlushDB(ctx)
		
		t.Log("🔧 Test 2: With Processor")
		proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
		proc.Start(ctx)
		defer proc.Stop()
		
		start2 := time.Now()
		for i := 0; i < 1000; i++ {
			msg, _ := translator.TranslateProduce("processed", 0, nil, make([]byte, 1024))
			processedMsg, _ := proc.ProcessMessage(ctx, msg)
			dragonflyStore.Store(ctx, processedMsg)
		}
		processedTime := time.Since(start2)
		processedThroughput := float64(1000) / processedTime.Seconds()
		
		t.Logf("  Throughput: %.0f msgs/sec", processedThroughput)
		
		overhead := (processedTime.Seconds() - directTime.Seconds()) / directTime.Seconds() * 100
		t.Logf("")
		t.Logf("📊 Processor Overhead: %.1f%%", overhead)
		t.Logf("  Direct:     %.0f msgs/sec", directThroughput)
		t.Logf("  Processed:  %.0f msgs/sec", processedThroughput)
		t.Logf("  Slowdown:   %.1fx", directThroughput/processedThroughput)
	})
	
	t.Run("AllocationProfile", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		
		// Measure allocations
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		for i := 0; i < 1000; i++ {
			translator.TranslateProduce("alloc-test", 0, nil, make([]byte, 1024))
		}
		
		runtime.ReadMemStats(&m2)
		
		allocPerMsg := (m2.TotalAlloc - m1.TotalAlloc) / 1000
		mallocsPerMsg := (m2.Mallocs - m1.Mallocs) / 1000
		
		t.Logf("🧮 Allocation Profile (per message):")
		t.Logf("  Bytes allocated: %d bytes", allocPerMsg)
		t.Logf("  Malloc calls:    %d", mallocsPerMsg)
		t.Logf("")
		
		if allocPerMsg > 5000 {
			t.Logf("⚠️ High allocation rate detected!")
			t.Logf("  Optimization needed: Use buffer pools")
		}
		
		if mallocsPerMsg > 20 {
			t.Logf("⚠️ Too many malloc calls!")
			t.Logf("  Optimization needed: Reduce allocations")
		}
	})
}

// BenchmarkOptimizationOpportunities benchmarks different optimization strategies
func BenchmarkOptimizationOpportunities(b *testing.B) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-bench-opt",
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
	
	b.Run("Current", func(b *testing.B) {
		translator := kafka.NewKafkaTranslator()
		proc := processor.NewMessageProcessor(processor.DefaultProcessorConfig())
		proc.Start(ctx)
		defer proc.Stop()
		
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		bridge := kafka.NewProcessorBridge(proc, kafkaStore)
		defer bridge.Stop()
		
		payload := make([]byte, 1024)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg, _ := translator.TranslateProduce("bench", 0, nil, payload)
			bridge.ProduceMessage(ctx, msg)
		}
		b.StopTimer()
		bridge.Stop()
		
		b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
	})
	
	b.Run("DirectDragonfly", func(b *testing.B) {
		payload := make([]byte, 1024)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := &types.PortaskMessage{
				ID:        types.MessageID(fmt.Sprintf("%d", i)),
				Topic:     types.TopicName("bench-direct"),
				Payload:   payload,
				Timestamp: time.Now().UnixNano(),
			}
			dragonflyStore.Store(ctx, msg)
		}
		
		b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
	})
}

