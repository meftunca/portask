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
	"github.com/meftunca/portask/pkg/types"
)

// ParallelBatchKafkaAdapter uses parallel batch writes
type ParallelBatchKafkaAdapter struct {
	ctx          context.Context
	store        *dragonfly.DragonflyStore
	subBatchSize int
}

func NewParallelBatchKafkaAdapter(ctx context.Context, store *dragonfly.DragonflyStore, subBatchSize int) *ParallelBatchKafkaAdapter {
	return &ParallelBatchKafkaAdapter{
		ctx:          ctx,
		store:        store,
		subBatchSize: subBatchSize,
	}
}

func (p *ParallelBatchKafkaAdapter) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	return p.store.StoreBatchParallel(ctx, batch, p.subBatchSize)
}

func (p *ParallelBatchKafkaAdapter) Store(ctx context.Context, msg *types.PortaskMessage) error {
	return p.store.Store(ctx, msg)
}

// Stub methods to satisfy interface
func (p *ParallelBatchKafkaAdapter) Fetch(ctx context.Context, topic types.TopicName, partition int32, offset int64, limit int) ([]*types.PortaskMessage, error) {
	return nil, nil
}
func (p *ParallelBatchKafkaAdapter) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	return nil, nil
}
func (p *ParallelBatchKafkaAdapter) Delete(ctx context.Context, messageID types.MessageID) error {
	return nil
}
func (p *ParallelBatchKafkaAdapter) DeleteBatch(ctx context.Context, messageIDs []types.MessageID) error {
	return nil
}
func (p *ParallelBatchKafkaAdapter) CreateTopic(ctx context.Context, topicInfo *types.TopicInfo) error {
	return nil
}
func (p *ParallelBatchKafkaAdapter) DeleteTopic(ctx context.Context, topic types.TopicName) error {
	return nil
}
func (p *ParallelBatchKafkaAdapter) GetTopicInfo(ctx context.Context, topic types.TopicName) (*types.TopicInfo, error) {
	return nil, nil
}
func (p *ParallelBatchKafkaAdapter) ListTopics(ctx context.Context) ([]*types.TopicInfo, error) {
	return nil, nil
}
func (p *ParallelBatchKafkaAdapter) TopicExists(ctx context.Context, topic types.TopicName) (bool, error) {
	return false, nil
}
func (p *ParallelBatchKafkaAdapter) GetPartitionInfo(ctx context.Context, topic types.TopicName, partition int32) (*types.PartitionInfo, error) {
	return nil, nil
}
func (p *ParallelBatchKafkaAdapter) GetPartitionCount(ctx context.Context, topic types.TopicName) (int32, error) {
	return 0, nil
}
func (p *ParallelBatchKafkaAdapter) GetLatestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}
func (p *ParallelBatchKafkaAdapter) GetEarliestOffset(ctx context.Context, topic types.TopicName, partition int32) (int64, error) {
	return 0, nil
}
func (p *ParallelBatchKafkaAdapter) CommitOffset(ctx context.Context, offset *types.ConsumerOffset) error {
	return nil
}
func (p *ParallelBatchKafkaAdapter) CommitOffsetBatch(ctx context.Context, offsets []*types.ConsumerOffset) error {
	return nil
}
func (p *ParallelBatchKafkaAdapter) GetOffset(ctx context.Context, consumerID types.ConsumerID, topic types.TopicName, partition int32) (*types.ConsumerOffset, error) {
	return nil, nil
}
func (p *ParallelBatchKafkaAdapter) GetConsumerOffsets(ctx context.Context, consumerID types.ConsumerID) ([]*types.ConsumerOffset, error) {
	return nil, nil
}
func (p *ParallelBatchKafkaAdapter) ListConsumers(ctx context.Context, topic types.TopicName) ([]types.ConsumerID, error) {
	return nil, nil
}
func (p *ParallelBatchKafkaAdapter) Ping(ctx context.Context) error {
	return nil
}
func (p *ParallelBatchKafkaAdapter) Stats(ctx context.Context) (*storage.StorageStats, error) {
	return nil, nil
}
func (p *ParallelBatchKafkaAdapter) Cleanup(ctx context.Context, retentionPolicy *storage.RetentionPolicy) error {
	return nil
}
func (p *ParallelBatchKafkaAdapter) Close() error {
	return nil
}

// TestParallelBatchWrite tests connection pool parallelization
func TestParallelBatchWrite(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000
	
	dfConfig := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		DB:        0,
		KeyPrefix: "portask-parallel-test",
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
	t.Logf("🔬 PARALLEL BATCH WRITE TEST")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Connection Pool: 1000 connections")
	t.Logf("Testing %d messages...", messageCount)
	t.Logf("")
	
	results := make(map[string]float64)
	
	// Test 1: Baseline (single pipeline)
	t.Run("Baseline_Single_Pipeline", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		// Use regular adapter (single pipeline)
		kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.HighThroughputConfig()
		asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("📦 Single Pipeline (current implementation)")
		
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
		
		t.Logf("   Throughput: %.0f msgs/sec", throughput)
	})
	
	time.Sleep(500 * time.Millisecond)
	
	// Test 2: Parallel with sub-batch size 50
	t.Run("Parallel_SubBatch_50", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		// Use parallel adapter
		parallelAdapter := NewParallelBatchKafkaAdapter(ctx, dragonflyStore, 50)
		
		config := processor.HighThroughputConfig()
		asyncWriter := processor.NewAsyncBatchWriter(parallelAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("🚀 Parallel (50 msgs/connection)")
		
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
		results["parallel_50"] = throughput
		
		t.Logf("   Throughput: %.0f msgs/sec", throughput)
	})
	
	time.Sleep(500 * time.Millisecond)
	
	// Test 3: Parallel with sub-batch size 25
	t.Run("Parallel_SubBatch_25", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		parallelAdapter := NewParallelBatchKafkaAdapter(ctx, dragonflyStore, 25)
		
		config := processor.HighThroughputConfig()
		asyncWriter := processor.NewAsyncBatchWriter(parallelAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("🚀 Parallel (25 msgs/connection)")
		
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
		results["parallel_25"] = throughput
		
		t.Logf("   Throughput: %.0f msgs/sec", throughput)
	})
	
	time.Sleep(500 * time.Millisecond)
	
	// Test 4: Parallel with sub-batch size 100
	t.Run("Parallel_SubBatch_100", func(t *testing.T) {
		dragonflyStore.GetClient().FlushDB(ctx)
		time.Sleep(100 * time.Millisecond)
		
		parallelAdapter := NewParallelBatchKafkaAdapter(ctx, dragonflyStore, 100)
		
		config := processor.HighThroughputConfig()
		asyncWriter := processor.NewAsyncBatchWriter(parallelAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("🚀 Parallel (100 msgs/connection)")
		
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
		
		t.Logf("   Throughput: %.0f msgs/sec", throughput)
	})
	
	// Summary
	t.Run("Summary", func(t *testing.T) {
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("📊 PARALLEL BATCH RESULTS")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")
		
		baseline := results["baseline"]
		
		t.Logf("Configuration      | Throughput        | vs Baseline")
		t.Logf("-------------------|-------------------|-------------")
		t.Logf("Baseline (single)  | %6.0f msgs/sec  | 0%%", baseline)
		
		for _, name := range []string{"parallel_25", "parallel_50", "parallel_100"} {
			throughput := results[name]
			improvement := ((throughput - baseline) / baseline) * 100
			
			marker := ""
			if improvement > 50 {
				marker = "🚀🚀"
			} else if improvement > 20 {
				marker = "🚀"
			} else if improvement > 0 {
				marker = "✅"
			} else {
				marker = "⚠️"
			}
			
			t.Logf("%-18s | %6.0f msgs/sec  | %+.0f%% %s",
				name, throughput, improvement, marker)
		}
		
		t.Logf("")
		
		// Find best
		var best string
		var bestThroughput float64
		for name, throughput := range results {
			if name == "baseline" {
				continue
			}
			if throughput > bestThroughput {
				bestThroughput = throughput
				best = name
			}
		}
		
		if bestThroughput > baseline {
			improvement := ((bestThroughput - baseline) / baseline) * 100
			t.Logf("🏆 Winner: %s", best)
			t.Logf("   Improvement: +%.0f%%", improvement)
			t.Logf("")
			t.Logf("💡 Parallel batch write WORKS!")
			t.Logf("   Connection pool parallelism increases throughput")
			t.Logf("   Multiple pipelines = better network utilization")
		} else {
			t.Logf("⚠️  No improvement detected")
			t.Logf("   Single pipeline might be sufficient")
		}
		
		t.Logf("")
	})
}
