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

// TestIntegratedParallelBatch tests AsyncBatchWriter with parallel batch writes enabled
func TestIntegratedParallelBatch(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000
	
	dfConfig := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		DB:        0,
		KeyPrefix: "portask-integrated-parallel",
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
	t.Logf("🚀 INTEGRATED PARALLEL BATCH TEST")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Messages: %d", messageCount)
	t.Logf("Message Size: 1KB")
	t.Logf("")
	
	results := make(map[string]float64)
	
	// Test 1: Parallel Writes DISABLED
	t.Run("Parallel_Disabled", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		// Disable parallel writes
		config := processor.HighThroughputConfig()
		config.EnableParallelWrites = false
		
		asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("Testing with parallel writes DISABLED...")
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		time.Sleep(200 * time.Millisecond)
		asyncWriter.Stop()
		
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		results["disabled"] = throughput
		
		t.Logf("  Throughput: %.0f msgs/sec", throughput)
	})
	
	time.Sleep(500 * time.Millisecond)
	
	// Test 2: Parallel Writes ENABLED (SubBatchSize = 200)
	t.Run("Parallel_200", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		// Enable parallel writes with SubBatchSize = 200
		config := processor.HighThroughputConfig()
		config.EnableParallelWrites = true
		config.SubBatchSize = 200
		
		asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("Testing with parallel writes ENABLED (SubBatchSize = 200)...")
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		time.Sleep(200 * time.Millisecond)
		asyncWriter.Stop()
		
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		results["parallel_200"] = throughput
		
		t.Logf("  Throughput: %.0f msgs/sec", throughput)
	})
	
	time.Sleep(500 * time.Millisecond)
	
	// Test 3: Parallel Writes ENABLED (SubBatchSize = 100)
	t.Run("Parallel_100", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		// Enable parallel writes with SubBatchSize = 100
		config := processor.HighThroughputConfig()
		config.EnableParallelWrites = true
		config.SubBatchSize = 100
		
		asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("Testing with parallel writes ENABLED (SubBatchSize = 100)...")
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		time.Sleep(200 * time.Millisecond)
		asyncWriter.Stop()
		
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		results["parallel_100"] = throughput
		
		t.Logf("  Throughput: %.0f msgs/sec", throughput)
	})
	
	// Summary
	t.Run("Summary", func(t *testing.T) {
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("📊 INTEGRATED PARALLEL RESULTS")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")
		
		baseline := results["disabled"]
		parallel200 := results["parallel_200"]
		parallel100 := results["parallel_100"]
		
		improvement200 := ((parallel200 - baseline) / baseline) * 100
		improvement100 := ((parallel100 - baseline) / baseline) * 100
		
		t.Logf("Configuration           | Throughput        | vs Disabled")
		t.Logf("------------------------|-------------------|-------------")
		t.Logf("Parallel DISABLED       | %6.0f msgs/sec  | 0%%", baseline)
		t.Logf("Parallel (SubBatch 200) | %6.0f msgs/sec  | %+.0f%% %s",
			parallel200, improvement200, getImprovementMarker(improvement200))
		t.Logf("Parallel (SubBatch 100) | %6.0f msgs/sec  | %+.0f%% %s",
			parallel100, improvement100, getImprovementMarker(improvement100))
		t.Logf("")
		
		bestThroughput := parallel200
		bestConfig := "SubBatchSize = 200"
		bestImprovement := improvement200
		
		if parallel100 > parallel200 {
			bestThroughput = parallel100
			bestConfig = "SubBatchSize = 100"
			bestImprovement = improvement100
		}
		
		t.Logf("🏆 Best Configuration: %s", bestConfig)
		t.Logf("   Throughput: %.0f msgs/sec", bestThroughput)
		t.Logf("   Improvement: +%.0f%%", bestImprovement)
		t.Logf("")
		
		if bestImprovement > 50 {
			t.Logf("✅ PARALLEL BATCH WRITES ARE HIGHLY EFFECTIVE!")
			t.Logf("   Connection pool parallelization provides major gains")
			t.Logf("   Production recommendation: ENABLE parallel writes")
		} else if bestImprovement > 20 {
			t.Logf("✅ Parallel batch writes provide significant improvement")
			t.Logf("   Recommended for production use")
		} else if bestImprovement > 0 {
			t.Logf("✅ Modest improvement from parallel writes")
		} else {
			t.Logf("⚠️  No improvement - parallel writes not effective")
		}
		
		t.Logf("")
		t.Logf("💡 Default Config: SubBatchSize = 200, Enabled = true")
		t.Logf("")
	})
}

func getImprovementMarker(improvement float64) string {
	if improvement > 100 {
		return "🚀🚀🚀"
	} else if improvement > 50 {
		return "🚀🚀"
	} else if improvement > 20 {
		return "🚀"
	} else if improvement > 0 {
		return "✅"
	}
	return "⚠️"
}

