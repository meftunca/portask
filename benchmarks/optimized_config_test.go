package benchmarks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
)

// TestOptimizedConfig tests the optimized configuration
func TestOptimizedConfig(t *testing.T) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-optimized",
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
	
	t.Run("OptimalConfig_Performance", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		// Use optimal config (32 shards, 100 batch size, 5ms flush)
		config := processor.DefaultParallelBatchWriterConfig()
		t.Logf("🔧 Optimal Config:")
		t.Logf("   Shards:        %d", config.NumShards)
		t.Logf("   Batch Size:    %d", config.BatchSize)
		t.Logf("   Flush Interval: %v", config.FlushInterval)
		t.Logf("")
		
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		messageCount := 50000
		payload := make([]byte, 1024)
		
		t.Logf("🚀 Sending %d messages...", messageCount)
		start := time.Now()
		
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		
		parallelWriter.Stop()
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		dataRate := float64(messageCount*1024) / duration.Seconds() / 1024 / 1024
		
		t.Logf("")
		t.Logf("✅ Results:")
		t.Logf("   Messages:    %d", messageCount)
		t.Logf("   Duration:    %v", duration)
		t.Logf("   Throughput:  %.0f msgs/sec 🚀", throughput)
		t.Logf("   Data Rate:   %.2f MB/s", dataRate)
		t.Logf("   Avg Latency: %.2f μs", float64(duration.Microseconds())/float64(messageCount))
		
		// Stats
		stats := parallelWriter.GetStats()
		t.Logf("")
		t.Logf("📊 Batch Stats:")
		t.Logf("   Total Batches: %d", stats.TotalBatches)
		t.Logf("   Avg Batch:     %.0f msgs", stats.AvgBatchSize)
		t.Logf("   Errors:        %d", stats.ErrorCount)
		
		// Compare with previous best (41K msgs/sec with 16 shards)
		previousBest := 41188.0
		improvement := (throughput - previousBest) / previousBest * 100
		
		t.Logf("")
		t.Logf("📈 Comparison:")
		t.Logf("   Previous Best: %.0f msgs/sec (16 shards, 1000 batch)", previousBest)
		t.Logf("   New Optimized: %.0f msgs/sec (32 shards, 100 batch)", throughput)
		if improvement > 0 {
			t.Logf("   Improvement:   +%.1f%% 🎉", improvement)
		} else {
			t.Logf("   Change:        %.1f%%", improvement)
		}
	})
	
	t.Run("CompareConfigurations", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		messageCount := 10000
		payload := make([]byte, 1024)
		
		configs := []struct {
			name       string
			shards     int
			batchSize  int
			flushMs    int
		}{
			{"Old Default", 8, 1000, 10},
			{"16 Shards (previous best)", 16, 1000, 10},
			{"NEW OPTIMAL", 32, 100, 5},
		}
		
		results := make([]float64, len(configs))
		
		for i, cfg := range configs {
			dragonflyStore.GetClient().FlushDB(ctx)
			time.Sleep(100 * time.Millisecond)
			
			kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
			storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
			
			config := &processor.ParallelBatchWriterConfig{
				NumShards:     cfg.shards,
				FlushInterval: time.Duration(cfg.flushMs) * time.Millisecond,
				BatchSize:     cfg.batchSize,
				MaxRetries:    3,
			}
			
			parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
			parallelWriter.Start(ctx)
			
			start := time.Now()
			for j := 0; j < messageCount; j++ {
				msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", j%20), 0, nil, payload)
				parallelWriter.Write(msg)
			}
			parallelWriter.Stop()
			
			duration := time.Since(start)
			throughput := float64(messageCount) / duration.Seconds()
			results[i] = throughput
		}
		
		t.Logf("")
		t.Logf("📊 Configuration Comparison (%d messages):", messageCount)
		t.Logf("─────────────────────────────────────────────────────")
		for i, cfg := range configs {
			speedup := results[i] / results[0]
			t.Logf("%-30s: %6.0f msgs/sec (%.2fx)", cfg.name, results[i], speedup)
		}
		t.Logf("─────────────────────────────────────────────────────")
		
		// Find best
		bestIdx := 0
		bestThroughput := results[0]
		for i, tp := range results {
			if tp > bestThroughput {
				bestThroughput = tp
				bestIdx = i
			}
		}
		
		t.Logf("")
		t.Logf("🏆 Winner: %s with %.0f msgs/sec!", configs[bestIdx].name, bestThroughput)
	})
	
	t.Run("StressTest_100K", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		// Use optimal config
		config := processor.HighThroughputConfig()
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		messageCount := 100000
		payload := make([]byte, 1024)
		
		t.Logf("🔥 STRESS TEST: %d messages", messageCount)
		start := time.Now()
		
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("stress-%d", i%100), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		
		parallelWriter.Stop()
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		dataRate := float64(messageCount*1024) / duration.Seconds() / 1024 / 1024
		
		t.Logf("")
		t.Logf("✅ STRESS TEST COMPLETE:")
		t.Logf("   Messages:    %d", messageCount)
		t.Logf("   Duration:    %v", duration)
		t.Logf("   Throughput:  %.0f msgs/sec 🚀", throughput)
		t.Logf("   Data Rate:   %.2f MB/s", dataRate)
		t.Logf("   Total Data:  %.2f MB", float64(messageCount*1024)/1024/1024)
		
		stats := parallelWriter.GetStats()
		t.Logf("")
		t.Logf("📊 Final Stats:")
		t.Logf("   Total Batches: %d", stats.TotalBatches)
		t.Logf("   Avg Batch:     %.0f msgs", stats.AvgBatchSize)
		t.Logf("   Errors:        %d", stats.ErrorCount)
		
		if throughput > 50000 {
			t.Logf("")
			t.Logf("🎉 AMAZING! Over 50K msgs/sec sustained!")
		}
	})
}

