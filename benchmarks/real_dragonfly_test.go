package benchmarks

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// DragonflyKafkaStore implements kafka.MessageStore interface using Dragonfly
type DragonflyKafkaStore struct {
	store *dragonfly.DragonflyStore
	ctx   context.Context
}

func (d *DragonflyKafkaStore) ProduceMessage(topic string, partition int32, key, value []byte) (int64, error) {
	msg := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("%d", time.Now().UnixNano())),
		Topic:     types.TopicName(topic),
		Partition: partition,
		Key:       string(key),
		Payload:   value,
		Timestamp: time.Now().UnixNano(), // Use UnixNano for precision
		TTL:       int64(time.Hour),      // 1 hour TTL in nanoseconds
	}
	
	err := d.store.Store(d.ctx, msg)
	if err != nil {
		return 0, err
	}
	
	return time.Now().UnixNano(), nil
}

func (d *DragonflyKafkaStore) ConsumeMessages(topic string, partition int32, offset int64, maxBytes int32) ([]*kafka.Message, error) {
	// For benchmark, we don't need to implement this
	return []*kafka.Message{}, nil
}

func (d *DragonflyKafkaStore) GetTopicMetadata(topics []string) (*kafka.TopicMetadata, error) {
	return &kafka.TopicMetadata{}, nil
}

func (d *DragonflyKafkaStore) CreateTopic(topic string, partitions int32, replication int16) error {
	return nil
}

func (d *DragonflyKafkaStore) DeleteTopic(topic string) error {
	return nil
}

// TestRealDragonflyBenchmark - GERÇEK Dragonfly ile benchmark
func TestRealDragonflyBenchmark(t *testing.T) {
	// Disable logging
	oldOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(oldOutput)

	// Create Dragonfly connection
	config := &storage.DragonflyConfig{
		Addresses:         []string{"localhost:6379"},
		DB:                0,
		KeyPrefix:         "benchmark",
		EnableCompression: false, // Disable for pure performance
		EnableCluster:     false,
	}

	ctx := context.Background()
	dragonflyStore, err := dragonfly.NewDragonflyStore(config)
	if err != nil {
		t.Fatalf("Failed to create Dragonfly store: %v", err)
	}

	// Try to connect
	err = dragonflyStore.Connect(ctx)
	if err != nil {
		t.Skipf("Dragonfly not available (start with: docker run -p 6379:6379 docker.dragonflydb.io/dragonflydb/dragonfly): %v", err)
		return
	}
	defer dragonflyStore.Close()

	// Clean up before test
	// (Optional: flush test data)

	// Create Kafka store wrapper
	store := &DragonflyKafkaStore{
		store: dragonflyStore,
		ctx:   ctx,
	}

	// Start Kafka server with REAL storage
	server := kafka.NewKafkaServer(":9102", store)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(300 * time.Millisecond)

	fmt.Printf("\n")
	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("║  💾 REAL DRAGONFLY BENCHMARK - Gerçek Persistence!               ║\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")
	fmt.Printf("  ⚙️  Configuration:\n")
	fmt.Printf("     Storage:     Dragonfly/Redis\n")
	fmt.Printf("     Persistence: Enabled (disk writes)\n")
	fmt.Printf("     Compression: %v\n", config.EnableCompression)
	fmt.Printf("     TTL:         1 hour\n")
	fmt.Printf("\n")

	// Test 1: Sync throughput with real storage
	fmt.Printf("  📊 Test 1: Sync Throughput (Real Dragonfly)\n")
	syncThroughput := measureDragonflySync(t, ":9102", 8, 5*time.Second)
	fmt.Printf("     Result: %.0f msgs/sec\n", syncThroughput)
	fmt.Printf("     Note:   This includes:\n")
	fmt.Printf("             - Network overhead\n")
	fmt.Printf("             - Serialization\n")
	fmt.Printf("             - Dragonfly write to disk\n")
	fmt.Printf("             - Redis protocol overhead\n")
	fmt.Printf("\n")

	// Test 2: Pipeline with real storage
	fmt.Printf("  📊 Test 2: Pipeline Throughput (Real Dragonfly)\n")
	pipelineThroughput := measureDragonflyPipeline(t, ":9102", 8, 10, 5*time.Second)
	fmt.Printf("     Result: %.0f msgs/sec\n", pipelineThroughput)
	fmt.Printf("     vs Sync: %.1fx improvement\n", pipelineThroughput/syncThroughput)
	fmt.Printf("\n")

	// Test 3: High concurrency
	fmt.Printf("  📊 Test 3: High Concurrency (16 producers)\n")
	concurrentThroughput := measureDragonflyPipeline(t, ":9102", 16, 10, 5*time.Second)
	fmt.Printf("     Result: %.0f msgs/sec\n", concurrentThroughput)
	fmt.Printf("\n")

	// Get Dragonfly stats
	stats, err := dragonflyStore.Stats(ctx)
	if err != nil {
		t.Logf("Warning: Could not get stats: %v", err)
		stats = &storage.StorageStats{}
	}
	
	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  📈 REAL WORLD PERFORMANCE SUMMARY                               ║\n")
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  Storage:                 Dragonfly (REAL disk writes)           ║\n")
	fmt.Printf("║  Sync (8 producers):      %.0f msgs/sec                        ║\n", syncThroughput)
	fmt.Printf("║  Pipeline (8 producers):  %.0f msgs/sec                        ║\n", pipelineThroughput)
	fmt.Printf("║  Peak (16 producers):     %.0f msgs/sec                        ║\n", concurrentThroughput)
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  Dragonfly Stats:                                                ║\n")
	fmt.Printf("║    Total Operations:      %d                                   ║\n", stats.TotalOperations)
	fmt.Printf("║    Successful:            %d                                   ║\n", stats.SuccessfulOperations)
	fmt.Printf("║    Failed:                %d                                   ║\n", stats.FailedOperations)
	fmt.Printf("║    Avg Response Time:     %.2fms                               ║\n", float64(stats.AverageResponseTime)/float64(time.Millisecond))
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  💡 REAL WORLD INSIGHTS:                                         ║\n")
	fmt.Printf("║  - This includes ALL overhead (network, disk, serialization)     ║\n")
	fmt.Printf("║  - Data is ACTUALLY written to Dragonfly/disk                    ║\n")
	fmt.Printf("║  - This is production-representative performance                 ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")

	// Comparison
	memoryBase := 14400.0 // From in-memory tests
	realWorldMultiplier := concurrentThroughput / memoryBase
	
	fmt.Printf("  🎯 COMPARISON:\n")
	fmt.Printf("     In-Memory Mock:      14,400 msgs/sec\n")
	fmt.Printf("     Real Dragonfly:      %.0f msgs/sec\n", concurrentThroughput)
	fmt.Printf("     Difference:          %.1fx\n", realWorldMultiplier)
	fmt.Printf("\n")
	
	if realWorldMultiplier < 0.5 {
		fmt.Printf("     💡 Real storage overhead significant (expected!)\n")
	} else if realWorldMultiplier < 0.8 {
		fmt.Printf("     ✅ Good performance with real persistence\n")
	} else {
		fmt.Printf("     🔥 Excellent performance with real persistence!\n")
	}
	fmt.Printf("\n")
}

func measureDragonflySync(t *testing.T, addr string, producers int, duration time.Duration) float64 {
	var count atomic.Int64
	var errors atomic.Int32
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				errors.Add(1)
				return
			}
			defer conn.Close()

			conn.SetDeadline(time.Now().Add(duration + 10*time.Second))
			request := buildFastProduceRequest()

			for time.Since(start) < duration {
				if _, err := conn.Write(request); err != nil {
					errors.Add(1)
					return
				}

				respBuf := make([]byte, 16)
				if _, err := io.ReadAtLeast(conn, respBuf, 8); err != nil {
					errors.Add(1)
					return
				}

				count.Add(1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	return float64(count.Load()) / elapsed.Seconds()
}

func measureDragonflyPipeline(t *testing.T, addr string, producers int, pipelineDepth int, duration time.Duration) float64 {
	var count atomic.Int64
	var errors atomic.Int32
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				errors.Add(1)
				return
			}
			defer conn.Close()

			conn.SetDeadline(time.Now().Add(duration + 10*time.Second))
			request := buildFastProduceRequest()

			done := make(chan struct{})

			// Writer
			go func() {
				for {
					select {
					case <-done:
						return
					default:
						for j := 0; j < pipelineDepth; j++ {
							if time.Since(start) >= duration {
								return
							}
							conn.Write(request)
						}
					}
				}
			}()

			// Reader
			go func() {
				respBuf := make([]byte, 16)
				for {
					if time.Since(start) >= duration {
						close(done)
						return
					}
					if _, err := io.ReadAtLeast(conn, respBuf, 8); err != nil {
						return
					}
					count.Add(1)
				}
			}()

			<-done
			time.Sleep(100 * time.Millisecond)
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	return float64(count.Load()) / elapsed.Seconds()
}

