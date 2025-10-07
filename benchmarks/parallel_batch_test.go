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

// TestParallelBatchWriter tests parallel batch writer performance
func TestParallelBatchWriter(t *testing.T) {
	// Setup Dragonfly
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-parallel-test",
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
	
	t.Log("✅ Connected to Dragonfly")
	
	t.Run("SingleVsParallel", func(t *testing.T) {
		translator := kafka.NewKafkaTranslator()
		messageCount := 10000
		payload := make([]byte, 1024) // 1KB
		
		// Test 1: Single batch writer (current)
		t.Log("🔧 Test 1: Single Batch Writer")
		kafkaStore1 := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter1 := &kafka.KafkaStorageAdapter{Storage: kafkaStore1}
		
		singleWriter := processor.NewBatchWriter(storageAdapter1, processor.DefaultBatchWriterConfig())
		singleWriter.Start(ctx)
		
		start1 := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce("single", 0, nil, payload)
			singleWriter.Write(msg)
		}
		singleWriter.Stop() // Force flush
		dur1 := time.Since(start1)
		throughput1 := float64(messageCount) / dur1.Seconds()
		
		t.Logf("  Throughput: %.0f msgs/sec", throughput1)
		t.Logf("  Duration:   %v", dur1)
		
		// Clear data
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		// Test 2: Parallel batch writer (new)
		t.Log("🔧 Test 2: Parallel Batch Writer (8 shards)")
		kafkaStore2 := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter2 := &kafka.KafkaStorageAdapter{Storage: kafkaStore2}
		
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter2, processor.DefaultParallelBatchWriterConfig())
		parallelWriter.Start(ctx)
		
		start2 := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("parallel-%d", i%10), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		parallelWriter.Stop() // Force flush all shards
		dur2 := time.Since(start2)
		throughput2 := float64(messageCount) / dur2.Seconds()
		
		t.Logf("  Throughput: %.0f msgs/sec", throughput2)
		t.Logf("  Duration:   %v", dur2)
		
		// Stats
		stats := parallelWriter.GetStats()
		t.Logf("  Shards:     %d", stats.NumShards)
		t.Logf("  Avg Batch:  %.0f msgs", stats.AvgBatchSize)
		
		// Comparison
		improvement := (throughput2 - throughput1) / throughput1 * 100
		t.Logf("")
		t.Logf("📊 Comparison:")
		t.Logf("   Single:       %.0f msgs/sec", throughput1)
		t.Logf("   Parallel:     %.0f msgs/sec", throughput2)
		t.Logf("   Improvement:  %.1f%% 🚀", improvement)
		t.Logf("   Speedup:      %.2fx", throughput2/throughput1)
		
		if improvement < 50 {
			t.Logf("⚠️ Warning: Less than 50%% improvement. Check shard distribution.")
		}
	})
	
	t.Run("ShardDistribution", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, processor.DefaultParallelBatchWriterConfig())
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		// Send messages to different topics
		messageCount := 10000
		payload := make([]byte, 1024)
		
		for i := 0; i < messageCount; i++ {
			topic := fmt.Sprintf("topic-%d", i%20) // 20 different topics
			msg, _ := translator.TranslateProduce(topic, 0, nil, payload)
			parallelWriter.Write(msg)
		}
		
		time.Sleep(50 * time.Millisecond) // Let it flush
		
		stats := parallelWriter.GetStats()
		t.Logf("📊 Shard Distribution:")
		
		for _, shardStat := range stats.ShardStats {
			percentage := float64(shardStat.MessageCount) / float64(messageCount) * 100
			t.Logf("  Shard %d: %d msgs (%.1f%%)", shardStat.ShardID, shardStat.MessageCount, percentage)
		}
		
		// Check if distribution is reasonably balanced (each shard should have ~12.5%)
		expectedPerShard := int64(messageCount / 8)
		tolerance := 0.3 // 30% tolerance
		
		for _, shardStat := range stats.ShardStats {
			deviation := float64(shardStat.MessageCount-expectedPerShard) / float64(expectedPerShard)
			if deviation > tolerance || deviation < -tolerance {
				t.Logf("⚠️ Shard %d is imbalanced (%.1f%% deviation)", shardStat.ShardID, deviation*100)
			}
		}
	})
	
	t.Run("HighConcurrency", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		// Increased shards for higher concurrency
		config := processor.DefaultParallelBatchWriterConfig()
		config.NumShards = 16 // 16 parallel writers
		config.BatchSize = 500 // Smaller batches for faster flush
		
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		messageCount := 50000
		payload := make([]byte, 1024)
		
		t.Logf("🚀 Sending %d messages with %d shards...", messageCount, config.NumShards)
		start := time.Now()
		
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("high-concurrency-%d", i%100), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		
		parallelWriter.Stop()
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		dataRate := float64(messageCount*1024) / duration.Seconds() / 1024 / 1024 // MB/s
		
		t.Logf("✅ Completed in %v", duration)
		t.Logf("📊 Results:")
		t.Logf("   Messages:    %d", messageCount)
		t.Logf("   Duration:    %v", duration)
		t.Logf("   Throughput:  %.0f msgs/sec", throughput)
		t.Logf("   Data Rate:   %.2f MB/s", dataRate)
		t.Logf("   Avg Latency: %.2f ms", float64(duration.Milliseconds())/float64(messageCount))
		
		stats := parallelWriter.GetStats()
		t.Logf("   Avg Batch:   %.0f msgs", stats.AvgBatchSize)
		t.Logf("   Total Batches: %d", stats.TotalBatches)
	})
}

// BenchmarkParallelBatchWriter benchmarks parallel batch writer
func BenchmarkParallelBatchWriter(b *testing.B) {
	dfConfig := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "portask-bench-parallel",
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
	
	dragonflyStore.GetClient().FlushDB(ctx)
	
	b.Run("8Shards", func(b *testing.B) {
		translator := kafka.NewKafkaTranslator()
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.DefaultParallelBatchWriterConfig()
		config.NumShards = 8
		
		parallelWriter := processor.NewParallelBatchWriter(storageAdapter, config)
		parallelWriter.Start(ctx)
		defer parallelWriter.Stop()
		
		payload := make([]byte, 1024)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("bench-%d", i%10), 0, nil, payload)
			parallelWriter.Write(msg)
		}
		b.StopTimer()
		parallelWriter.Stop()
		
		b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "msgs/sec")
	})
}

