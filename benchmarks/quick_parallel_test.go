package benchmarks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// TestQuickParallelBatch tests parallel batch performance directly
func TestQuickParallelBatch(t *testing.T) {
	ctx := context.Background()
	
	dfConfig := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		DB:        0,
		KeyPrefix: "portask-quick-parallel",
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
	
	// Prepare test batch
	batchSize := 5000
	payload := make([]byte, 1024)
	messages := make([]*types.PortaskMessage, batchSize)
	
	for i := 0; i < batchSize; i++ {
		messages[i] = &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("msg-%d", i)),
			Topic:     "test-topic",
			Partition: 0,
			Payload:   payload,
			Timestamp: time.Now().UnixNano(),
			TTL:       int64(time.Hour),
			Metadata:  make(map[string]string),
			Headers:   make(types.MessageHeaders),
		}
	}
	
	batch := types.NewMessageBatch(messages)
	
	t.Logf("")
	t.Logf("🔬 PARALLEL BATCH PERFORMANCE")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Batch Size: %d messages", batchSize)
	t.Logf("Message Size: 1KB")
	t.Logf("Connection Pool: 1000 connections")
	t.Logf("")
	
	results := make(map[string]float64)
	
	// Test 1: Single Pipeline
	t.Run("Single_Pipeline", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		start := time.Now()
		err := dragonflyStore.StoreBatch(ctx, batch)
		duration := time.Since(start)
		
		if err != nil {
			t.Fatalf("StoreBatch failed: %v", err)
		}
		
		throughput := float64(batchSize) / duration.Seconds()
		results["single"] = throughput
		
		t.Logf("Single Pipeline:")
		t.Logf("  Duration:   %v", duration)
		t.Logf("  Throughput: %.0f msgs/sec", throughput)
	})
	
	time.Sleep(200 * time.Millisecond)
	
	// Test 2: Parallel (sub-batch 25)
	t.Run("Parallel_25", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		start := time.Now()
		err := dragonflyStore.StoreBatchParallel(ctx, batch, 25)
		duration := time.Since(start)
		
		if err != nil {
			t.Fatalf("StoreBatchParallel failed: %v", err)
		}
		
		throughput := float64(batchSize) / duration.Seconds()
		results["parallel_25"] = throughput
		
		t.Logf("Parallel (25 msgs/conn):")
		t.Logf("  Duration:   %v", duration)
		t.Logf("  Throughput: %.0f msgs/sec", throughput)
	})
	
	time.Sleep(200 * time.Millisecond)
	
	// Test 3: Parallel (sub-batch 50)
	t.Run("Parallel_50", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		start := time.Now()
		err := dragonflyStore.StoreBatchParallel(ctx, batch, 50)
		duration := time.Since(start)
		
		if err != nil {
			t.Fatalf("StoreBatchParallel failed: %v", err)
		}
		
		throughput := float64(batchSize) / duration.Seconds()
		results["parallel_50"] = throughput
		
		t.Logf("Parallel (50 msgs/conn):")
		t.Logf("  Duration:   %v", duration)
		t.Logf("  Throughput: %.0f msgs/sec", throughput)
	})
	
	time.Sleep(200 * time.Millisecond)
	
	// Test 4: Parallel (sub-batch 100)
	t.Run("Parallel_100", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		start := time.Now()
		err := dragonflyStore.StoreBatchParallel(ctx, batch, 100)
		duration := time.Since(start)
		
		if err != nil {
			t.Fatalf("StoreBatchParallel failed: %v", err)
		}
		
		throughput := float64(batchSize) / duration.Seconds()
		results["parallel_100"] = throughput
		
		t.Logf("Parallel (100 msgs/conn):")
		t.Logf("  Duration:   %v", duration)
		t.Logf("  Throughput: %.0f msgs/sec", throughput)
	})
	
	// Summary
	t.Run("Summary", func(t *testing.T) {
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("📊 RESULTS COMPARISON")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")
		
		baseline := results["single"]
		
		t.Logf("Method             | Throughput         | vs Single")
		t.Logf("-------------------|--------------------|-----------")
		t.Logf("Single Pipeline    | %7.0f msgs/sec   | 0%%", baseline)
		
		for _, config := range []struct{name string; key string}{
			{"Parallel (25)", "parallel_25"},
			{"Parallel (50)", "parallel_50"},
			{"Parallel (100)", "parallel_100"},
		} {
			throughput := results[config.key]
			improvement := ((throughput - baseline) / baseline) * 100
			
			marker := ""
			if improvement > 100 {
				marker = "🚀🚀🚀"
			} else if improvement > 50 {
				marker = "🚀🚀"
			} else if improvement > 20 {
				marker = "🚀"
			} else if improvement > 0 {
				marker = "✅"
			} else {
				marker = "⚠️"
			}
			
			t.Logf("%-18s | %7.0f msgs/sec   | %+.0f%% %s",
				config.name, throughput, improvement, marker)
		}
		
		t.Logf("")
		
		// Find best
		var best string
		var bestThroughput float64
		for name, throughput := range results {
			if name == "single" {
				continue
			}
			if throughput > bestThroughput {
				bestThroughput = throughput
				best = name
			}
		}
		
		improvement := ((bestThroughput - baseline) / baseline) * 100
		
		t.Logf("🏆 Best: %s", best)
		t.Logf("   Throughput: %.0f msgs/sec", bestThroughput)
		t.Logf("   Improvement: +%.0f%%", improvement)
		t.Logf("")
		
		if improvement > 20 {
			t.Logf("✅ Parallel batch write is EFFECTIVE!")
			t.Logf("   Connection pool parallelism significantly improves throughput")
		} else if improvement > 0 {
			t.Logf("✅ Parallel batch write provides modest improvement")
		} else {
			t.Logf("⚠️  No improvement - single pipeline is sufficient")
		}
		
		t.Logf("")
	})
}

