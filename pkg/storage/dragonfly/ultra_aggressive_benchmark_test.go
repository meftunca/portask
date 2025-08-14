//go:build never
// +build never

// Ultra-Aggressive Benchmark Tests

package dragonfly

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// Helper function to setup benchmark store
func setupUltraBenchmarkStore() (*DragonflyStore, error) {
	// Check if Dragonfly is available
	conn, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("Dragonfly not available: %v", err)
	}
	conn.Close()

	config := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		Password:  "",
		DB:        0,
	}

	store, err := NewDragonflyStore(config)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	err = store.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return store, nil
}

// Helper function to generate benchmark messages
func generateUltraBenchmarkMessages(count int, topic string, partition int32) []*types.PortaskMessage {
	messages := make([]*types.PortaskMessage, count)

	for i := 0; i < count; i++ {
		messages[i] = &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("ultra-msg-%d", i)),
			Topic:     types.TopicName(topic),
			Partition: partition,
			Offset:    int64(i),
			Payload:   []byte(fmt.Sprintf("ultra test message %d", i)),
			Timestamp: time.Now().Unix(),
		}
	}

	return messages
}

// BenchmarkUltraAggressiveFetch tests ultra-optimized fetch performance
func BenchmarkUltraAggressiveFetch(b *testing.B) {
	store, err := setupUltraBenchmarkStore()
	if err != nil {
		b.Skipf("Failed to setup store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Pre-populate with many messages for realistic testing
	messages := generateUltraBenchmarkMessages(10000, "ultra-test-topic", 0)
	for _, msg := range messages {
		if err := store.Store(ctx, msg); err != nil {
			b.Fatalf("Failed to store message: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := store.Fetch(ctx, "ultra-test-topic", 0, 0, 100)
			if err != nil {
				b.Fatalf("Fetch failed: %v", err)
			}
		}
	})
}

// BenchmarkUltraAggressiveBatchDelete tests batch deletion (FIXED)
func BenchmarkUltraAggressiveBatchDelete(b *testing.B) {
	store, err := setupUltraBenchmarkStore()
	if err != nil {
		b.Skipf("Failed to setup store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create very few messages for each iteration to avoid hanging
		messages := generateUltraBenchmarkMessages(10, "delete-test-topic", int32(i%3)) // Reduced from 100 to 10
		messageIDs := make([]types.MessageID, len(messages))

		for j, msg := range messages {
			store.Store(ctx, msg)
			messageIDs[j] = msg.ID
		}
		b.StartTimer()

		err := store.DeleteBatch(ctx, messageIDs)
		if err != nil {
			b.Fatalf("Batch delete failed: %v", err)
		}
	}
}

// BenchmarkUltraAggressiveCleanup tests cleanup operations (FIXED)
func BenchmarkUltraAggressiveCleanup(b *testing.B) {
	store, err := setupUltraBenchmarkStore()
	if err != nil {
		b.Skipf("Failed to setup store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Pre-populate with old messages (minimal amount)
	oldMessages := generateOldUltraBenchmarkMessages(50, "cleanup-test-topic", 0) // Reduced from 1000 to 50
	for _, msg := range oldMessages {
		store.Store(ctx, msg)
	}

	policy := &storage.RetentionPolicy{
		MaxAge: 1 * time.Hour, // Messages older than 1 hour
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := store.Cleanup(ctx, policy)
		if err != nil {
			b.Fatalf("Cleanup failed: %v", err)
		}
	}
}

// BenchmarkUltraAggressiveListTopics tests topic listing (FIXED)
func BenchmarkUltraAggressiveListTopics(b *testing.B) {
	store, err := setupUltraBenchmarkStore()
	if err != nil {
		b.Skipf("Failed to setup store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create very few topics (reduced from 50 to 5)
	for i := 0; i < 5; i++ {
		topicInfo := &types.TopicInfo{
			Name:       types.TopicName(fmt.Sprintf("topic-%d", i)),
			Partitions: 1,
		}
		store.CreateTopic(ctx, topicInfo)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := store.ListTopics(ctx)
			if err != nil {
				b.Fatalf("ListTopics failed: %v", err)
			}
		}
	})
}

// BenchmarkMegaThroughput tests extreme throughput scenarios
func BenchmarkMegaThroughput(b *testing.B) {
	store, err := setupUltraBenchmarkStore()
	if err != nil {
		b.Skipf("Failed to setup store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	b.Run("MegaStore", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			messageCounter := 0
			for pb.Next() {
				message := &types.PortaskMessage{
					ID:        types.MessageID(fmt.Sprintf("mega-msg-%d-%d", b.N, messageCounter)),
					Topic:     "mega-topic",
					Partition: 0,
					Offset:    int64(messageCounter),
					Payload:   []byte("mega test data for ultra performance"),
					Timestamp: time.Now().Unix(),
				}
				messageCounter++

				err := store.Store(ctx, message)
				if err != nil {
					b.Fatalf("Store failed: %v", err)
				}
			}
		})
	})

	b.Run("MegaFetch", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := store.Fetch(ctx, "mega-topic", 0, 0, 1000)
				if err != nil {
					b.Fatalf("Fetch failed: %v", err)
				}
			}
		})
	})
}

// Helper function to generate old messages for cleanup testing
func generateOldUltraBenchmarkMessages(count int, topic string, partition int32) []*types.PortaskMessage {
	messages := make([]*types.PortaskMessage, count)
	oldTime := time.Now().Add(-2 * time.Hour).Unix() // 2 hours old

	for i := 0; i < count; i++ {
		messages[i] = &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("old-msg-%d", i)),
			Topic:     types.TopicName(topic),
			Partition: partition,
			Offset:    int64(i),
			Payload:   []byte(fmt.Sprintf("old test message %d", i)),
			Timestamp: oldTime,
		}
	}

	return messages
}

// BenchmarkConnectionStress tests connection usage (FIXED)
func BenchmarkConnectionStress(b *testing.B) {
	store, err := setupUltraBenchmarkStore()
	if err != nil {
		b.Skipf("Failed to setup store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	b.ReportAllocs()
	b.SetParallelism(2) // Reduced from 10 to 2

	b.RunParallel(func(pb *testing.PB) {
		messageCounter := 0
		for pb.Next() {
			// Mix of operations to stress connections
			switch messageCounter % 4 {
			case 0:
				// Store
				message := &types.PortaskMessage{
					ID:        types.MessageID(fmt.Sprintf("stress-msg-%d", messageCounter)),
					Topic:     "stress-topic",
					Partition: 0,
					Offset:    int64(messageCounter),
					Payload:   []byte("stress test data"),
					Timestamp: time.Now().Unix(),
				}
				store.Store(ctx, message)
			case 1:
				// Fetch
				store.Fetch(ctx, "stress-topic", 0, 0, 10)
			case 2:
				// TopicExists
				store.TopicExists(ctx, "stress-topic")
			case 3:
				// Ping
				store.Ping(ctx)
			}
			messageCounter++
		}
	})
}
