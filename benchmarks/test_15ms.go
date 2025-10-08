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

func TestQuick15ms(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000

	dfConfig := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		DB:        0,
		KeyPrefix: "portask-15ms-test",
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

	kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
	storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}

	config := &processor.ParallelBatchWriterConfig{
		NumShards:     32,
		FlushInterval: 15 * time.Millisecond,
		BatchSize:     500,
		MaxRetries:    3,
	}

	asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
	asyncWriter.Start(ctx)

	start := time.Now()
	for i := 0; i < messageCount; i++ {
		msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
		asyncWriter.Write(msg)
		memory.PutMessage(msg)
	}

	time.Sleep(300 * time.Millisecond)
	asyncWriter.Stop()

	duration := time.Since(start)
	throughput := float64(messageCount) / duration.Seconds()

	metrics := asyncWriter.GetMetrics()
	avgBatch := float64(messageCount) / float64(metrics.TotalBatchesWritten.Load())

	t.Logf("")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Results with 15ms flush interval:")
	t.Logf("  Throughput:    %.0f msgs/sec", throughput)
	t.Logf("  Avg Batch:     %.0f msgs", avgBatch)
	t.Logf("  Batch Fill:    %.1f%%", (avgBatch/500)*100)
	t.Logf("")
	t.Logf("Comparison:")
	t.Logf("  10ms: ~210K msgs/sec")
	t.Logf("  15ms: %.0f msgs/sec", throughput)
	t.Logf("  20ms: ~59K msgs/sec")

	t.Logf("")
	if throughput > 190000 {
		t.Logf("  ✅ 15ms: Good alternative to 10ms")
		t.Logf("  Trade-off: Slightly less throughput, slightly more latency")
	} else if throughput > 150000 {
		t.Logf("  ⚠️  15ms: Acceptable but 10ms is better")
	} else {
		t.Logf("  ❌ 15ms: Too slow, use 10ms")
	}
	t.Logf("")
}
