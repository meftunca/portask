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
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/badgerdb"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// BadgerKafkaAdapter adapts BadgerDB to Kafka MessageStore interface
type BadgerKafkaAdapter struct {
	ctx   context.Context
	store *badgerdb.BadgerStore
}

func NewBadgerKafkaAdapter(ctx context.Context, store *badgerdb.BadgerStore) *BadgerKafkaAdapter {
	return &BadgerKafkaAdapter{ctx: ctx, store: store}
}

func (b *BadgerKafkaAdapter) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	msg := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Partition: partition,
		Key:       string(key),
		Payload:   value,
		Timestamp: time.Now().UnixNano(),
		TTL:       int64(time.Hour),
		Headers:   make(types.MessageHeaders),
		Metadata:  make(map[string]string),
	}

	if err := b.store.Store(b.ctx, msg); err != nil {
		return 0, err
	}

	return msg.Timestamp, nil
}

func (b *BadgerKafkaAdapter) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	return []*kafka.Message{}, nil
}

func (b *BadgerKafkaAdapter) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{}, nil
}

func (b *BadgerKafkaAdapter) CreateTopic(topic string, partitions int32, replication int16) error {
	return nil
}

func (b *BadgerKafkaAdapter) DeleteTopic(topic string) error {
	return nil
}

// TestPhase9LocalStorage compares network storage (Dragonfly) vs local storage (BadgerDB)
func TestPhase9LocalStorage(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 100000 // Large test for BadgerDB
	
	t.Logf("")
	t.Logf("🏔️  Phase 9: Local Storage (BadgerDB) vs Network Storage")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Testing %d messages...", messageCount)
	t.Logf("")
	
	// Test 1: Dragonfly (Network Storage)
	var dragonflyThroughput float64
	
	t.Run("Dragonfly_Network", func(t *testing.T) {
		dfConfig := &storage.DragonflyConfig{
			Addresses:         []string{"localhost:6379"},
			DB:                0,
			KeyPrefix:         "portask-phase9-df",
			EnableCompression: false,
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
		
		config := processor.HighThroughputConfig() // Now using batch size 500
		asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("📡 Testing Network Storage (Dragonfly)...")
		t.Logf("   Location: Remote (localhost:6379)")
		t.Logf("   Protocol: TCP/IP")
		t.Logf("")
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		time.Sleep(500 * time.Millisecond) // Wait for async
		asyncWriter.Stop()
		
		duration := time.Since(start)
		dragonflyThroughput = float64(messageCount) / duration.Seconds()
		
		t.Logf("✅ Dragonfly Results:")
		t.Logf("   Duration:    %v", duration)
		t.Logf("   Throughput:  %.0f msgs/sec", dragonflyThroughput)
		t.Logf("   Data Rate:   %.2f MB/s", float64(messageCount*1024)/duration.Seconds()/1024/1024)
		t.Logf("")
	})
	
	time.Sleep(time.Second) // Cool down
	
	// Test 2: BadgerDB (Local Storage)
	var badgerThroughput float64
	
	t.Run("BadgerDB_Local", func(t *testing.T) {
		// Create BadgerDB store
		testDir := "./test_badger_data"
		defer os.RemoveAll(testDir) // Cleanup
		
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
		
		t.Logf("💾 Testing Local Storage (BadgerDB)...")
		t.Logf("   Location: Local disk (%s)", testDir)
		t.Logf("   Protocol: Direct I/O (pure Go)")
		t.Logf("")
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		time.Sleep(500 * time.Millisecond) // Wait for async
		asyncWriter.Stop()
		
		duration := time.Since(start)
		badgerThroughput = float64(messageCount) / duration.Seconds()
		
		metrics := badgerStore.GetMetrics()
		
		t.Logf("✅ BadgerDB Results:")
		t.Logf("   Duration:    %v", duration)
		t.Logf("   Throughput:  %.0f msgs/sec", badgerThroughput)
		t.Logf("   Data Rate:   %.2f MB/s", float64(messageCount*1024)/duration.Seconds()/1024/1024)
		t.Logf("   Written:     %d messages", metrics["messages_written"])
		t.Logf("   Disk Size:   %.2f MB", float64(metrics["bytes_written"])/1024/1024)
		t.Logf("")
	})
	
	// Comparison
	t.Run("Comparison", func(t *testing.T) {
		if dragonflyThroughput == 0 || badgerThroughput == 0 {
			t.Skip("One or both tests skipped")
			return
		}
		
		speedup := badgerThroughput / dragonflyThroughput
		improvement := (badgerThroughput - dragonflyThroughput) / dragonflyThroughput * 100
		
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("📊 STORAGE COMPARISON: Network vs Local")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")
		t.Logf("Dragonfly (Network):  %.0f msgs/sec", dragonflyThroughput)
		t.Logf("BadgerDB (Local):     %.0f msgs/sec", badgerThroughput)
		t.Logf("")
		t.Logf("Speedup:              %.2fx", speedup)
		t.Logf("Improvement:          +%.1f%%", improvement)
		t.Logf("")
		
		if speedup > 5 {
			t.Logf("🎉 LOCAL STORAGE IS 5X+ FASTER!")
			t.Logf("   Local disk I/O eliminates network latency")
		} else if speedup > 2 {
			t.Logf("✅ Local storage provides significant benefit")
		} else if speedup > 1 {
			t.Logf("✅ Local storage is faster")
		} else {
			t.Logf("⚠️  Network storage competitive (good network?)")
		}
		
		t.Logf("")
		t.Logf("💡 Trade-offs:")
		t.Logf("")
		t.Logf("Network Storage (Dragonfly):")
		t.Logf("  ✅ Distributed (multiple servers)")
		t.Logf("  ✅ Shared state")
		t.Logf("  ✅ High availability")
		t.Logf("  ⚠️  Network latency")
		t.Logf("  ⚠️  Limited by bandwidth")
		t.Logf("")
		t.Logf("Local Storage (BadgerDB):")
		t.Logf("  ✅ Ultra-fast (no network)")
		t.Logf("  ✅ High throughput (pure Go)")
		t.Logf("  ✅ Low latency")
		t.Logf("  ⚠️  Single server only")
		t.Logf("  ⚠️  No distributed state")
		t.Logf("")
		t.Logf("🎯 Recommendation:")
		if speedup > 3 {
			t.Logf("  Use BadgerDB/RocksDB for maximum performance")
			t.Logf("  Use Dragonfly for distributed scenarios")
		} else {
			t.Logf("  Dragonfly is good enough for most use cases")
			t.Logf("  Consider BadgerDB only if throughput critical")
		}
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	})
}

