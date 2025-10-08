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

// TestFullSystemParallel tests the full system with parallel batch writes
func TestFullSystemParallel(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000
	
	dfConfig := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		DB:        0,
		KeyPrefix: "portask-full-parallel",
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
	t.Logf("🚀 FULL SYSTEM PARALLEL TEST")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Messages: %d", messageCount)
	t.Logf("Message Size: 1KB")
	t.Logf("Connection Pool: 1000 connections")
	t.Logf("")
	
	results := make(map[string]float64)
	
	// Test 1: Baseline (single pipeline)
	t.Run("Baseline", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.HighThroughputConfig()
		asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("Baseline (single pipeline)...")
		
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
		results["baseline"] = throughput
		
		t.Logf("  Throughput: %.0f msgs/sec", throughput)
	})
	
	time.Sleep(500 * time.Millisecond)
	
	// Test 2: Parallel (sub-batch 100)
	t.Run("Parallel_100", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		// Use parallel adapter
		parallelAdapter := NewParallelBatchKafkaAdapter(ctx, dragonflyStore, 100)
		
		config := processor.HighThroughputConfig()
		asyncWriter := processor.NewAsyncBatchWriter(parallelAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("Parallel (100 msgs/connection)...")
		
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
		results["parallel"] = throughput
		
		t.Logf("  Throughput: %.0f msgs/sec", throughput)
	})
	
	// Summary
	t.Run("Summary", func(t *testing.T) {
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("📊 FULL SYSTEM RESULTS")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")
		
		baseline := results["baseline"]
		parallel := results["parallel"]
		improvement := ((parallel - baseline) / baseline) * 100
		
		t.Logf("Configuration        | Throughput")
		t.Logf("---------------------|-------------------")
		t.Logf("Baseline (single)    | %.0f msgs/sec", baseline)
		t.Logf("Parallel (100)       | %.0f msgs/sec (+%.0f%%)", parallel, improvement)
		t.Logf("")
		
		if improvement > 50 {
			t.Logf("🚀🚀 HUGE IMPROVEMENT!")
			t.Logf("   Connection pool parallelization is VERY effective")
			t.Logf("   Expected real-world: 200K → %.0fK msgs/sec", parallel/1000)
		} else if improvement > 20 {
			t.Logf("🚀 SIGNIFICANT IMPROVEMENT!")
			t.Logf("   Parallel batch write provides substantial gains")
		} else if improvement > 0 {
			t.Logf("✅ Modest improvement")
		} else {
			t.Logf("⚠️  No improvement in full system")
		}
		
		t.Logf("")
		t.Logf("💡 Recommendation:")
		if improvement > 20 {
			t.Logf("   Deploy with parallel batch writes enabled")
			t.Logf("   Sub-batch size: 100 messages")
			t.Logf("   Expected production: %.0fK+ msgs/sec", parallel/1000)
		} else {
			t.Logf("   Single pipeline might be sufficient for current workload")
		}
		t.Logf("")
	})
}

