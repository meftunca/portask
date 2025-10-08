package benchmarks

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/memory"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage/badgerdb"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/storage"
)

func TestQuickBadger(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000
	
	// Test BadgerDB
	testDir := "./quick_badger_data"
	defer os.RemoveAll(testDir)
	
	badgerStore, err := badgerdb.NewBadgerStore(&badgerdb.Config{
		DataDir: testDir,
	})
	if err != nil {
		t.Fatalf("Failed to create BadgerDB: %v", err)
	}
	defer badgerStore.Close()
	
	kafkaStore := NewBadgerKafkaAdapter(ctx, badgerStore)
	storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
	
	config := processor.HighThroughputConfig()
	asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
	asyncWriter.Start(ctx)
	
	t.Logf("Testing BadgerDB with %d messages...", messageCount)
	start := time.Now()
	for i := 0; i < messageCount; i++ {
		msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
		asyncWriter.Write(msg)
		memory.PutMessage(msg)
	}
	
	time.Sleep(800 * time.Millisecond) // Wait for async
	asyncWriter.Stop()
	
	duration := time.Since(start)
	throughput := float64(messageCount) / duration.Seconds()
	
	t.Logf("")
	t.Logf("✅ BadgerDB Results:")
	t.Logf("   Messages:    %d", messageCount)
	t.Logf("   Duration:    %v", duration)
	t.Logf("   Throughput:  %.0f msgs/sec", throughput)
	t.Logf("")
}

func TestQuickDragonfly(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000
	
	dfConfig := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		DB:        0,
		KeyPrefix: "portask-quick-df",
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
	
	config := processor.HighThroughputConfig()
	asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
	asyncWriter.Start(ctx)
	
	t.Logf("Testing Dragonfly with %d messages...", messageCount)
	start := time.Now()
	for i := 0; i < messageCount; i++ {
		msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
		asyncWriter.Write(msg)
		memory.PutMessage(msg)
	}
	
	time.Sleep(800 * time.Millisecond) // Wait for async
	asyncWriter.Stop()
	
	duration := time.Since(start)
	throughput := float64(messageCount) / duration.Seconds()
	
	t.Logf("")
	t.Logf("✅ Dragonfly Results:")
	t.Logf("   Messages:    %d", messageCount)
	t.Logf("   Duration:    %v", duration)
	t.Logf("   Throughput:  %.0f msgs/sec", throughput)
	t.Logf("")
}

