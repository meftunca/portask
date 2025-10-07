package benchmarks

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
)

// TestAdvancedProfiling performs comprehensive profiling to find next bottlenecks
func TestAdvancedProfiling(t *testing.T) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-advanced-profile",
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
	
	t.Run("CPUProfile_ParallelWriter", func(t *testing.T) {
		// Start CPU profiling
		cpuFile, _ := os.Create("cpu_parallel.prof")
		defer cpuFile.Close()
		pprof.StartCPUProfile(cpuFile)
		defer pprof.StopCPUProfile()
		
		// Setup
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.DefaultParallelBatchWriterConfig()
		config.NumShards = 16
		
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		// Workload
		messageCount := 50000
		payload := make([]byte, 1024)
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%20), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		parallelWriter.Stop()
		
		duration := time.Since(start)
		t.Logf("✅ CPU Profile: %d msgs in %v (%.0f msgs/sec)", messageCount, duration, float64(messageCount)/duration.Seconds())
		t.Logf("📁 Profile: cpu_parallel.prof")
		t.Logf("   Run: go tool pprof -http=:8080 cpu_parallel.prof")
	})
	
	t.Run("MemoryProfile_ParallelWriter", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		
		// Setup
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.DefaultParallelBatchWriterConfig()
		config.NumShards = 16
		
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		// Baseline memory
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		// Workload
		messageCount := 50000
		payload := make([]byte, 1024)
		
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%20), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		parallelWriter.Stop()
		
		// Final memory
		runtime.GC()
		runtime.ReadMemStats(&m2)
		
		// Memory profile
		memFile, _ := os.Create("mem_parallel.prof")
		pprof.WriteHeapProfile(memFile)
		memFile.Close()
		
		// Analysis
		totalAlloc := m2.TotalAlloc - m1.TotalAlloc
		allocPerMsg := totalAlloc / uint64(messageCount)
		mallocs := m2.Mallocs - m1.Mallocs
		mallocsPerMsg := mallocs / uint64(messageCount)
		
		t.Logf("🧮 Memory Profile:")
		t.Logf("   Total Allocated: %d MB", totalAlloc/1024/1024)
		t.Logf("   Per Message:     %d bytes", allocPerMsg)
		t.Logf("   Mallocs Total:   %d", mallocs)
		t.Logf("   Mallocs/Msg:     %d", mallocsPerMsg)
		t.Logf("   Heap Inuse:      %d MB", (m2.HeapInuse-m1.HeapInuse)/1024/1024)
		t.Logf("")
		
		if allocPerMsg > 3000 {
			t.Logf("⚠️ High allocation rate: %d bytes/msg", allocPerMsg)
			t.Logf("   Recommendation: Use more buffer pools")
		}
		
		if mallocsPerMsg > 10 {
			t.Logf("⚠️ High malloc rate: %d mallocs/msg", mallocsPerMsg)
			t.Logf("   Recommendation: Reduce allocations, reuse objects")
		}
		
		t.Logf("📁 Profile: mem_parallel.prof")
		t.Logf("   Run: go tool pprof -http=:8080 mem_parallel.prof")
	})
	
	t.Run("GoroutineProfile", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.DefaultParallelBatchWriterConfig()
		config.NumShards = 16
		
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		
		// Send some messages
		payload := make([]byte, 1024)
		for i := 0; i < 1000; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%20), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		
		time.Sleep(100 * time.Millisecond)
		
		// Goroutine profile
		goroutineFile, _ := os.Create("goroutine.prof")
		pprof.Lookup("goroutine").WriteTo(goroutineFile, 1)
		goroutineFile.Close()
		
		numGoroutines := runtime.NumGoroutine()
		t.Logf("🔄 Goroutine Count: %d", numGoroutines)
		
		if numGoroutines > 100 {
			t.Logf("⚠️ High goroutine count: %d", numGoroutines)
			t.Logf("   Check for goroutine leaks")
		}
		
		parallelWriter.Stop()
		
		t.Logf("📁 Profile: goroutine.prof")
	})
	
	t.Run("ExecutionTrace", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		
		// Start trace
		traceFile, _ := os.Create("trace.out")
		defer traceFile.Close()
		trace.Start(traceFile)
		defer trace.Stop()
		
		// Setup
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.DefaultParallelBatchWriterConfig()
		config.NumShards = 8
		
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		
		// Workload (smaller for trace)
		messageCount := 10000
		payload := make([]byte, 1024)
		
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%10), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		
		parallelWriter.Stop()
		
		t.Logf("📁 Trace: trace.out")
		t.Logf("   Run: go tool trace trace.out")
	})
	
	t.Run("DetailedLatencyBreakdown", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		
		translator := kafka.NewKafkaTranslator()
		
		messageCount := 1000
		payload := make([]byte, 1024)
		
		// Measure each component
		var (
			translateTime   time.Duration
			msgCreateTime   time.Duration
			dragonflyTime   time.Duration
		)
		
		for i := 0; i < messageCount; i++ {
			// 1. Translation
			t1 := time.Now()
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%10), 0, nil, payload)
			translateTime += time.Since(t1)
			
			// 2. Message creation (what translator does)
			t2 := time.Now()
			_ = msg
			msgCreateTime += time.Since(t2)
			
			// 3. Dragonfly write
			t3 := time.Now()
			dragonflyStore.Store(ctx, msg)
			dragonflyTime += time.Since(t3)
		}
		
		totalTime := translateTime + msgCreateTime + dragonflyTime
		
		t.Logf("📊 Latency Breakdown (per message):")
		t.Logf("   Translation:     %.2f μs (%.1f%%)", float64(translateTime.Microseconds())/float64(messageCount), float64(translateTime)/float64(totalTime)*100)
		t.Logf("   Msg Creation:    %.2f μs (%.1f%%)", float64(msgCreateTime.Microseconds())/float64(messageCount), float64(msgCreateTime)/float64(totalTime)*100)
		t.Logf("   Dragonfly Write: %.2f μs (%.1f%%)", float64(dragonflyTime.Microseconds())/float64(messageCount), float64(dragonflyTime)/float64(totalTime)*100)
		t.Logf("   Total:           %.2f μs", float64(totalTime.Microseconds())/float64(messageCount))
		t.Logf("")
		
		// Identify bottleneck
		if dragonflyTime > totalTime/2 {
			t.Logf("🎯 Bottleneck: Dragonfly Write (%.1f%%)", float64(dragonflyTime)/float64(totalTime)*100)
			t.Logf("   Solutions:")
			t.Logf("   • Already using parallel batch writer ✅")
			t.Logf("   • Consider: Pipeline writes")
			t.Logf("   • Consider: Batch compression")
		}
		
		if translateTime > totalTime/4 {
			t.Logf("⚠️ Translation overhead: %.1f%%", float64(translateTime)/float64(totalTime)*100)
			t.Logf("   Solutions:")
			t.Logf("   • Pre-allocate message structs")
			t.Logf("   • Use sync.Pool for messages")
		}
	})
	
	t.Run("AllocationHotspots", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		
		translator := kafka.NewKafkaTranslator()
		
		// Measure allocations in different operations
		var m1, m2, m3 runtime.MemStats
		
		// 1. Translation allocations
		runtime.GC()
		runtime.ReadMemStats(&m1)
		for i := 0; i < 1000; i++ {
			translator.TranslateProduce("topic", 0, nil, make([]byte, 1024))
		}
		runtime.ReadMemStats(&m2)
		
		translateAlloc := m2.TotalAlloc - m1.TotalAlloc
		translateMallocs := m2.Mallocs - m1.Mallocs
		
		// 2. Message creation allocations
		runtime.GC()
		runtime.ReadMemStats(&m1)
		for i := 0; i < 1000; i++ {
			msg, _ := translator.TranslateProduce("topic", 0, nil, make([]byte, 1024))
			_ = msg
		}
		runtime.ReadMemStats(&m3)
		
		msgCreateAlloc := m3.TotalAlloc - m1.TotalAlloc
		msgCreateMallocs := m3.Mallocs - m1.Mallocs
		
		t.Logf("🔥 Allocation Hotspots:")
		t.Logf("")
		t.Logf("Translation:")
		t.Logf("   Bytes:   %d bytes/msg", translateAlloc/1000)
		t.Logf("   Mallocs: %d mallocs/msg", translateMallocs/1000)
		t.Logf("")
		t.Logf("Message Creation:")
		t.Logf("   Bytes:   %d bytes/msg", msgCreateAlloc/1000)
		t.Logf("   Mallocs: %d mallocs/msg", msgCreateMallocs/1000)
		t.Logf("")
		
		// Recommendations
		if translateAlloc/1000 > 2000 {
			t.Logf("💡 Optimize Translation:")
			t.Logf("   • Use buffer pools for payload")
			t.Logf("   • Reuse message structs")
			t.Logf("   • Pre-allocate metadata map")
		}
		
		if msgCreateMallocs/1000 > 5 {
			t.Logf("💡 Reduce Allocations:")
			t.Logf("   • Object pooling (sync.Pool)")
			t.Logf("   • String interning for topics")
			t.Logf("   • Reuse byte slices")
		}
	})
}

// TestOptimizationImpact tests specific optimizations
func TestOptimizationImpact(t *testing.T) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-opt-impact",
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
	
	t.Run("ShardCount_Impact", func(t *testing.T) {
		shardCounts := []int{4, 8, 16, 32}
		
		for _, numShards := range shardCounts {
			dragonflyStore.GetClient().FlushDB(ctx)
			
			translator := kafka.NewKafkaTranslator()
			kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
			storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
			
			config := processor.DefaultParallelBatchWriterConfig()
			config.NumShards = numShards
			
			parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
			parallelWriter.Start(ctx)
			
			messageCount := 10000
			payload := make([]byte, 1024)
			
			start := time.Now()
			for i := 0; i < messageCount; i++ {
				msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%20), 0, nil, payload)
				parallelWriter.Write(msg)
			}
			parallelWriter.Stop()
			
			duration := time.Since(start)
			throughput := float64(messageCount) / duration.Seconds()
			
			t.Logf("%2d shards: %.0f msgs/sec (%v)", numShards, throughput, duration)
		}
	})
	
	t.Run("BatchSize_Impact", func(t *testing.T) {
		batchSizes := []int{100, 500, 1000, 2000, 5000}
		
		for _, batchSize := range batchSizes {
			dragonflyStore.GetClient().FlushDB(ctx)
			
			translator := kafka.NewKafkaTranslator()
			kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
			storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
			
			config := processor.DefaultParallelBatchWriterConfig()
			config.NumShards = 16
			config.BatchSize = batchSize
			
			parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
			parallelWriter.Start(ctx)
			
			messageCount := 10000
			payload := make([]byte, 1024)
			
			start := time.Now()
			for i := 0; i < messageCount; i++ {
				msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%20), 0, nil, payload)
				parallelWriter.Write(msg)
			}
			parallelWriter.Stop()
			
			duration := time.Since(start)
			throughput := float64(messageCount) / duration.Seconds()
			
			t.Logf("Batch %4d: %.0f msgs/sec (%v)", batchSize, throughput, duration)
		}
	})
}

