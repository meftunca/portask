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

func TestQuick20ms(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000
	
	dfConfig := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		DB:        0,
		KeyPrefix: "portask-20ms-test",
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
	
	dragonflyStore.GetClient().FlushDB(ctx)
	
	// Test with 20ms flush interval
	kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
	storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
	
	config := &processor.ParallelBatchWriterConfig{
		NumShards:     32,
		FlushInterval: 20 * time.Millisecond, // Testing 20ms
		BatchSize:     500,
		MaxRetries:    3,
	}
	
	asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
	asyncWriter.Start(ctx)
	
	t.Logf("Testing with 20ms flush interval...")
	
	start := time.Now()
	for i := 0; i < messageCount; i++ {
		msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
		asyncWriter.Write(msg)
		memory.PutMessage(msg)
	}
	
	time.Sleep(800 * time.Millisecond) // Much longer wait for 20ms
	asyncWriter.Stop()
	
	duration := time.Since(start)
	throughput := float64(messageCount) / duration.Seconds()
	
	metrics := asyncWriter.GetMetrics()
	avgBatch := float64(messageCount) / float64(metrics.TotalBatchesWritten.Load())
	
	t.Logf("")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Results with 20ms flush interval:")
	t.Logf("  Duration:      %v", duration)
	t.Logf("  Throughput:    %.0f msgs/sec", throughput)
	t.Logf("  Batches:       %d", metrics.TotalBatchesWritten.Load())
	t.Logf("  Avg Batch:     %.0f msgs", avgBatch)
	t.Logf("  Batch Fill:    %.1f%% of 500", (avgBatch/500)*100)
	t.Logf("")
	t.Logf("Comparison:")
	t.Logf("  10ms: ~200-220K msgs/sec (from Phase 8)")
	t.Logf("  20ms: %.0f msgs/sec (this test)", throughput)
	
	diff := throughput - 210000 // Approximate 10ms baseline
	pct := (diff / 210000) * 100
	
	t.Logf("")
	if pct > -5 && pct < 5 {
		t.Logf("  Result: Similar performance (%.1f%% difference)", pct)
		t.Logf("  Conclusion: 20ms is acceptable, slightly higher latency")
	} else if pct < -5 {
		t.Logf("  Result: %.1f%% slower", pct)
		t.Logf("  Conclusion: 10ms is better")
	} else {
		t.Logf("  Result: %.1f%% faster!", pct)
		t.Logf("  Conclusion: 20ms might be optimal")
	}
	t.Logf("")
}

