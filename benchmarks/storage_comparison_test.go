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
	"github.com/meftunca/portask/pkg/storage/rocksdb"
	"github.com/meftunca/portask/pkg/types"
)

// RocksDBKafkaAdapter adapts RocksDB to Kafka MessageStore interface
type RocksDBKafkaAdapter struct {
	ctx   context.Context
	store *rocksdb.RocksDBStore
}

func NewRocksDBKafkaAdapter(ctx context.Context, store *rocksdb.RocksDBStore) *RocksDBKafkaAdapter {
	return &RocksDBKafkaAdapter{ctx: ctx, store: store}
}

func (r *RocksDBKafkaAdapter) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
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

	if err := r.store.Store(r.ctx, msg); err != nil {
		return 0, err
	}

	return msg.Timestamp, nil
}

func (r *RocksDBKafkaAdapter) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	return []*kafka.Message{}, nil
}

func (r *RocksDBKafkaAdapter) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{}, nil
}

func (r *RocksDBKafkaAdapter) CreateTopic(topic string, partitions int32, replication int16) error {
	return nil
}

func (r *RocksDBKafkaAdapter) DeleteTopic(topic string) error {
	return nil
}

// TestStorageComparison compares all storage backends
func TestStorageComparison(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000
	
	results := make(map[string]float64)
	
	t.Logf("")
	t.Logf("🏆 STORAGE BACKEND COMPARISON")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Testing %d messages (1KB each)...", messageCount)
	t.Logf("")
	
	// Test 1: Dragonfly (Network Storage)
	t.Run("Dragonfly", func(t *testing.T) {
		dfConfig := &storage.DragonflyConfig{
			Addresses: []string{"localhost:6379"},
			DB:        0,
			KeyPrefix: "portask-storage-comp",
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
		
		t.Logf("📡 Dragonfly (Network)")
		t.Logf("   Type:     Redis-compatible, distributed")
		t.Logf("   Location: localhost:6379 (TCP/IP)")
		t.Logf("")
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		time.Sleep(600 * time.Millisecond)
		asyncWriter.Stop()
		
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		results["Dragonfly"] = throughput
		
		t.Logf("   Duration:    %v", duration)
		t.Logf("   Throughput:  %.0f msgs/sec", throughput)
		t.Logf("")
	})
	
	time.Sleep(time.Second)
	
	// Test 2: BadgerDB (Local Storage - Pure Go)
	t.Run("BadgerDB", func(t *testing.T) {
		testDir := "./storage_comp_badger"
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
		
		t.Logf("💎 BadgerDB (Local - Pure Go)")
		t.Logf("   Type:     LSM tree, pure Go")
		t.Logf("   Location: %s (local disk)", testDir)
		t.Logf("")
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		time.Sleep(600 * time.Millisecond)
		asyncWriter.Stop()
		
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		results["BadgerDB"] = throughput
		
		metrics := badgerStore.GetMetrics()
		
		t.Logf("   Duration:    %v", duration)
		t.Logf("   Throughput:  %.0f msgs/sec", throughput)
		t.Logf("   Disk Size:   %.2f MB", float64(metrics["bytes_written"])/1024/1024)
		t.Logf("")
	})
	
	time.Sleep(time.Second)
	
	// Test 3: RocksDB (Local Storage - C++)
	t.Run("RocksDB", func(t *testing.T) {
		testDir := "./storage_comp_rocksdb"
		defer os.RemoveAll(testDir)
		
		rocksdbStore, err := rocksdb.NewRocksDBStore(&rocksdb.Config{
			DataDir:           testDir,
			WriteBufferSize:   64 * 1024 * 1024,
			DisableWAL:        false, // Keep durability
			EnableCompression: false,
		})
		if err != nil {
			t.Skipf("RocksDB not available: %v", err)
			return
		}
		defer rocksdbStore.Close()
		
		kafkaStore := NewRocksDBKafkaAdapter(ctx, rocksdbStore)
		storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}
		
		config := processor.HighThroughputConfig()
		asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
		asyncWriter.Start(ctx)
		
		t.Logf("🪨 RocksDB (Local - C++)")
		t.Logf("   Type:     LSM tree, Facebook's RocksDB")
		t.Logf("   Location: %s (local disk)", testDir)
		t.Logf("")
		
		start := time.Now()
		for i := 0; i < messageCount; i++ {
			msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
			asyncWriter.Write(msg)
			memory.PutMessage(msg)
		}
		
		time.Sleep(600 * time.Millisecond)
		asyncWriter.Stop()
		
		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()
		results["RocksDB"] = throughput
		
		metrics := rocksdbStore.GetMetrics()
		
		t.Logf("   Duration:    %v", duration)
		t.Logf("   Throughput:  %.0f msgs/sec", throughput)
		t.Logf("   Disk Size:   %.2f MB", float64(metrics["bytes_written"])/1024/1024)
		t.Logf("")
	})
	
	// Comparison Summary
	t.Run("Summary", func(t *testing.T) {
		if len(results) < 2 {
			t.Skip("Not enough results for comparison")
			return
		}
		
		// Find best performer
		var bestName string
		var bestThroughput float64
		
		for name, throughput := range results {
			if throughput > bestThroughput {
				bestThroughput = throughput
				bestName = name
			}
		}
		
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("📊 FINAL COMPARISON")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")
		
		// Sort and display results
		backends := []string{"Dragonfly", "BadgerDB", "RocksDB"}
		for _, name := range backends {
			throughput, exists := results[name]
			if !exists {
				continue
			}
			
			marker := "  "
			if name == bestName {
				marker = "🏆"
			}
			
			percentage := (throughput / bestThroughput) * 100
			t.Logf("  %s %-12s: %6.0f msgs/sec (%.0f%%)", marker, name, throughput, percentage)
		}
		
		t.Logf("")
		t.Logf("Winner: %s (%.0f msgs/sec)", bestName, bestThroughput)
		t.Logf("")
		
		// Analysis
		t.Logf("💡 Analysis:")
		t.Logf("")
		
		dragonflyScore := results["Dragonfly"]
		badgerScore := results["BadgerDB"]
		rocksScore := results["RocksDB"]
		
		if dragonflyScore > 0 && badgerScore > 0 {
			ratio := dragonflyScore / badgerScore
			if ratio > 0.95 && ratio < 1.05 {
				t.Logf("  ✅ Network ≈ Local: Modern networks are fast!")
				t.Logf("     Dragonfly competitive with local storage")
			} else if dragonflyScore > badgerScore {
				t.Logf("  🚀 Network FASTER: Dragonfly optimizations FTW")
			} else {
				t.Logf("  💾 Local FASTER: Disk I/O wins")
			}
		}
		
		if rocksScore > 0 && badgerScore > 0 {
			ratio := rocksScore / badgerScore
			if ratio > 1.1 {
				t.Logf("  ⚡ RocksDB > BadgerDB: C++ speed advantage")
			} else if ratio < 0.9 {
				t.Logf("  🦡 BadgerDB > RocksDB: Pure Go efficiency")
			} else {
				t.Logf("  🤝 RocksDB ≈ BadgerDB: Both excellent")
			}
		}
		
		t.Logf("")
		t.Logf("📝 Recommendations:")
		t.Logf("")
		
		t.Logf("Use Dragonfly if:")
		t.Logf("  ✅ Distributed system needed")
		t.Logf("  ✅ Shared state across servers")
		t.Logf("  ✅ High availability required")
		t.Logf("  ✅ Already using Redis ecosystem")
		t.Logf("")
		
		t.Logf("Use BadgerDB if:")
		t.Logf("  ✅ Pure Go stack preferred")
		t.Logf("  ✅ No C dependencies wanted")
		t.Logf("  ✅ Embedded scenarios")
		t.Logf("  ✅ Single-server deployment")
		t.Logf("")
		
		if rocksScore > 0 {
			t.Logf("Use RocksDB if:")
			t.Logf("  ✅ Maximum local performance needed")
			t.Logf("  ✅ C++ dependencies acceptable")
			t.Logf("  ✅ Battle-tested stability preferred")
			t.Logf("  ✅ Used by Facebook/LinkedIn scale")
			t.Logf("")
		}
		
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")
	})
}

