package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// BenchmarkConfig holds benchmark configuration
type BenchmarkConfig struct {
	MessageCount       int
	ConcurrentUsers    int
	MessageSize        int
	TopicCount         int
	BatchSize          int
	TestDuration       time.Duration
	ConnectionPoolSize int
}

// DefaultBenchmarkConfig returns default benchmark configuration
func DefaultBenchmarkConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		MessageCount:       1000,
		ConcurrentUsers:    10,
		MessageSize:        256,
		TopicCount:         5,
		BatchSize:          100,
		TestDuration:       30 * time.Second,
		ConnectionPoolSize: 20,
	}
}

// BenchmarkResult holds benchmark results
type BenchmarkResult struct {
	TotalMessages     int64
	TotalErrors       int64
	Duration          time.Duration
	MessagesPerSecond float64
	ErrorRate         float64
	AvgLatency        time.Duration
	MinLatency        time.Duration
	MaxLatency        time.Duration
	P95Latency        time.Duration
	MemoryUsage       int64
	StorageSize       int64
}

// LatencyTracker tracks operation latencies
type LatencyTracker struct {
	latencies []time.Duration
	mu        sync.Mutex
}

func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{
		latencies: make([]time.Duration, 0, 10000),
	}
}

func (lt *LatencyTracker) Record(latency time.Duration) {
	lt.mu.Lock()
	lt.latencies = append(lt.latencies, latency)
	lt.mu.Unlock()
}

func (lt *LatencyTracker) GetStats() (avg, min, max, p95 time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.latencies) == 0 {
		return 0, 0, 0, 0
	}

	// Sort latencies for percentile calculation
	sorted := make([]time.Duration, len(lt.latencies))
	copy(sorted, lt.latencies)

	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calculate stats
	var total time.Duration
	for _, lat := range sorted {
		total += lat
	}
	avg = total / time.Duration(len(sorted))
	min = sorted[0]
	max = sorted[len(sorted)-1]
	p95Index := int(float64(len(sorted)) * 0.95)
	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}
	p95 = sorted[p95Index]

	return avg, min, max, p95
}

// Benchmark Mock Storage Performance
func BenchmarkMockStoragePerformance(b *testing.B) {
	configs := []*BenchmarkConfig{
		{MessageCount: 100, ConcurrentUsers: 1, MessageSize: 64, TopicCount: 3},
		{MessageCount: 1000, ConcurrentUsers: 5, MessageSize: 256, TopicCount: 5},
		{MessageCount: 5000, ConcurrentUsers: 10, MessageSize: 512, TopicCount: 10},
		{MessageCount: 10000, ConcurrentUsers: 20, MessageSize: 1024, TopicCount: 15},
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("Mock_Messages%d_Users%d_Size%d",
			config.MessageCount, config.ConcurrentUsers, config.MessageSize), func(b *testing.B) {

			storage := NewMockMessageStore()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				runStorageBenchmark(b, storage, config)
			}
		})
	}
}

// Benchmark Dragonfly Storage Performance
func BenchmarkDragonflyStoragePerformance(b *testing.B) {
	// Check Dragonfly availability
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 3})
	ctx := context.Background()
	if err := testClient.Ping(ctx).Err(); err != nil {
		b.Skipf("Dragonfly not available: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	configs := []*BenchmarkConfig{
		{MessageCount: 100, ConcurrentUsers: 1, MessageSize: 64, TopicCount: 3},
		{MessageCount: 1000, ConcurrentUsers: 5, MessageSize: 256, TopicCount: 5},
		{MessageCount: 5000, ConcurrentUsers: 10, MessageSize: 512, TopicCount: 10},
		{MessageCount: 10000, ConcurrentUsers: 20, MessageSize: 1024, TopicCount: 15},
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("Dragonfly_Messages%d_Users%d_Size%d",
			config.MessageCount, config.ConcurrentUsers, config.MessageSize), func(b *testing.B) {

			// Create Dragonfly storage
			storageConfig := &storage.DragonflyStorageConfig{
				Addr: "localhost:6379", Password: "", DB: 3,
				PoolSize: 20, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
			}

			dragonflyStorage, err := storage.NewDragonflyStorage(storageConfig)
			require.NoError(b, err)
			defer dragonflyStorage.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Clear database for each iteration
				testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 3})
				testClient.FlushDB(ctx)
				testClient.Close()

				runStorageBenchmark(b, dragonflyStorage, config)
			}
		})
	}
}

// Run storage benchmark
func runStorageBenchmark(b *testing.B, store storage.MessageStore, config *BenchmarkConfig) {
	ctx := context.Background()

	// Safety checks to prevent division by zero
	if config.ConcurrentUsers <= 0 {
		b.Fatalf("ConcurrentUsers must be greater than 0, got: %d", config.ConcurrentUsers)
	}
	if config.TopicCount <= 0 {
		b.Fatalf("TopicCount must be greater than 0, got: %d", config.TopicCount)
	}

	var wg sync.WaitGroup
	messagesPerUser := config.MessageCount / config.ConcurrentUsers

	startTime := time.Now()

	for userID := 0; userID < config.ConcurrentUsers; userID++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()

			for i := 0; i < messagesPerUser; i++ {
				message := &types.PortaskMessage{
					ID:        types.MessageID(fmt.Sprintf("bench-user%d-msg%d", uid, i)),
					Topic:     types.TopicName(fmt.Sprintf("benchmark.topic%d", uid%config.TopicCount)),
					Payload:   generatePayload(config.MessageSize),
					Timestamp: time.Now().UnixNano(),
				}

				if err := store.Store(ctx, message); err != nil {
					b.Errorf("Store failed: %v", err)
				}
			}
		}(userID)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Report custom metrics
	messagesPerSecond := float64(config.MessageCount) / duration.Seconds()
	b.ReportMetric(messagesPerSecond, "msg/sec")
	b.ReportMetric(duration.Seconds(), "duration_sec")
}

// Benchmark Network Protocol Performance
func BenchmarkNetworkProtocolPerformance(b *testing.B) {
	configs := []*BenchmarkConfig{
		{MessageCount: 100, ConcurrentUsers: 1, MessageSize: 64, TopicCount: 3},
		{MessageCount: 1000, ConcurrentUsers: 5, MessageSize: 256, TopicCount: 5},
		{MessageCount: 2000, ConcurrentUsers: 10, MessageSize: 512, TopicCount: 8},
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("Network_Messages%d_Users%d_Size%d",
			config.MessageCount, config.ConcurrentUsers, config.MessageSize), func(b *testing.B) {

			// Use mock storage for network benchmarks to focus on network performance
			storage := NewMockMessageStore()
			codecManager := &serialization.CodecManager{}
			protocolHandler := NewJSONCompatibleProtocolHandler(codecManager, storage)

			serverConfig := &ServerConfig{
				Address: "127.0.0.1", Port: 0, MaxConnections: config.ConcurrentUsers + 5,
				ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
				Network: "tcp", ReadBufferSize: 4096, WriteBufferSize: 4096,
				WorkerPoolSize: config.ConcurrentUsers,
			}

			server := NewTCPServer(serverConfig, protocolHandler)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			go func() { server.Start(ctx) }()
			time.Sleep(200 * time.Millisecond)

			addr := server.GetAddr()
			require.NotNil(b, addr, "Server should start successfully")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				runNetworkBenchmark(b, addr.String(), config)
			}

			server.Stop(ctx)
		})
	}
}

// Run network benchmark
func runNetworkBenchmark(b *testing.B, serverAddr string, config *BenchmarkConfig) {
	var wg sync.WaitGroup
	latencyTracker := NewLatencyTracker()
	var totalErrors int64

	messagesPerUser := config.MessageCount / config.ConcurrentUsers

	for userID := 0; userID < config.ConcurrentUsers; userID++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()

			conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
			if err != nil {
				atomic.AddInt64(&totalErrors, 1)
				return
			}
			defer conn.Close()

			for i := 0; i < messagesPerUser; i++ {
				start := time.Now()

				message := map[string]interface{}{
					"type":  "publish",
					"topic": fmt.Sprintf("benchmark.topic%d", uid%config.TopicCount),
					"data":  string(generatePayload(config.MessageSize)),
				}

				jsonBytes, err := json.Marshal(message)
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
					continue
				}

				_, err = conn.Write(append(jsonBytes, '\n'))
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
					continue
				}

				latency := time.Since(start)
				latencyTracker.Record(latency)
			}
		}(userID)
	}

	wg.Wait()

	// Report metrics
	avg, min, max, p95 := latencyTracker.GetStats()
	b.ReportMetric(float64(avg.Nanoseconds())/1000000, "avg_latency_ms")
	b.ReportMetric(float64(min.Nanoseconds())/1000000, "min_latency_ms")
	b.ReportMetric(float64(max.Nanoseconds())/1000000, "max_latency_ms")
	b.ReportMetric(float64(p95.Nanoseconds())/1000000, "p95_latency_ms")
	b.ReportMetric(float64(totalErrors), "total_errors")
}

// Benchmark End-to-End Performance with Dragonfly
func BenchmarkEndToEndDragonflyPerformance(b *testing.B) {
	// Check Dragonfly availability
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 4})
	ctx := context.Background()
	if err := testClient.Ping(ctx).Err(); err != nil {
		b.Skipf("Dragonfly not available: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	configs := []*BenchmarkConfig{
		{MessageCount: 500, ConcurrentUsers: 5, MessageSize: 256, TopicCount: 5},
		{MessageCount: 1000, ConcurrentUsers: 10, MessageSize: 512, TopicCount: 8},
		{MessageCount: 2000, ConcurrentUsers: 15, MessageSize: 1024, TopicCount: 12},
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("E2E_Dragonfly_Messages%d_Users%d_Size%d",
			config.MessageCount, config.ConcurrentUsers, config.MessageSize), func(b *testing.B) {

			// Create Dragonfly storage
			storageConfig := &storage.DragonflyStorageConfig{
				Addr: "localhost:6379", Password: "", DB: 4,
				PoolSize: 30, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
			}

			dragonflyStorage, err := storage.NewDragonflyStorage(storageConfig)
			require.NoError(b, err)
			defer dragonflyStorage.Close()

			codecManager := &serialization.CodecManager{}
			protocolHandler := NewJSONCompatibleProtocolHandler(codecManager, dragonflyStorage)

			serverConfig := &ServerConfig{
				Address: "127.0.0.1", Port: 0, MaxConnections: config.ConcurrentUsers + 10,
				ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second,
				Network: "tcp", ReadBufferSize: 8192, WriteBufferSize: 8192,
				WorkerPoolSize: config.ConcurrentUsers + 5,
			}

			server := NewTCPServer(serverConfig, protocolHandler)
			serverCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			go func() { server.Start(serverCtx) }()
			time.Sleep(300 * time.Millisecond)

			addr := server.GetAddr()
			require.NotNil(b, addr, "Server should start successfully")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Clear database for each iteration
				testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 4})
				testClient.FlushDB(ctx)
				testClient.Close()

				runEndToEndBenchmark(b, addr.String(), dragonflyStorage, config)
			}

			server.Stop(serverCtx)
		})
	}
}

// Run end-to-end benchmark with verification
func runEndToEndBenchmark(b *testing.B, serverAddr string, store storage.MessageStore, config *BenchmarkConfig) {
	ctx := context.Background()
	var wg sync.WaitGroup
	latencyTracker := NewLatencyTracker()
	var totalErrors int64
	var totalMessages int64

	messagesPerUser := config.MessageCount / config.ConcurrentUsers
	startTime := time.Now()

	// Send phase
	for userID := 0; userID < config.ConcurrentUsers; userID++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()

			conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
			if err != nil {
				atomic.AddInt64(&totalErrors, 1)
				return
			}
			defer conn.Close()

			for i := 0; i < messagesPerUser; i++ {
				msgStart := time.Now()

				message := map[string]interface{}{
					"type":  "publish",
					"topic": fmt.Sprintf("e2e.topic%d", uid%config.TopicCount),
					"data":  string(generatePayload(config.MessageSize)),
				}

				jsonBytes, err := json.Marshal(message)
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
					continue
				}

				_, err = conn.Write(append(jsonBytes, '\n'))
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
					continue
				}

				atomic.AddInt64(&totalMessages, 1)
				latency := time.Since(msgStart)
				latencyTracker.Record(latency)

				// Small delay to avoid overwhelming
				time.Sleep(time.Millisecond)
			}
		}(userID)
	}

	wg.Wait()
	sendDuration := time.Since(startTime)

	// Wait for processing
	time.Sleep(1 * time.Second)

	// Verification phase - check if messages are actually stored
	verificationStart := time.Now()
	var storedCount int64

	for topicID := 0; topicID < config.TopicCount; topicID++ {
		topicName := fmt.Sprintf("e2e.topic%d", topicID)
		messages, err := store.Fetch(ctx, types.TopicName(topicName), 0, 0, 1000)
		if err == nil {
			storedCount += int64(len(messages))
		}
	}
	verificationDuration := time.Since(verificationStart)

	// Report metrics
	avg, _, _, p95 := latencyTracker.GetStats()
	messagesPerSecond := float64(totalMessages) / sendDuration.Seconds()
	errorRate := float64(totalErrors) / float64(totalMessages) * 100
	storageSuccessRate := float64(storedCount) / float64(totalMessages) * 100

	b.ReportMetric(messagesPerSecond, "msg/sec")
	b.ReportMetric(errorRate, "error_rate_%")
	b.ReportMetric(storageSuccessRate, "storage_success_%")
	b.ReportMetric(float64(avg.Nanoseconds())/1000000, "avg_latency_ms")
	b.ReportMetric(float64(p95.Nanoseconds())/1000000, "p95_latency_ms")
	b.ReportMetric(sendDuration.Seconds(), "send_duration_sec")
	b.ReportMetric(verificationDuration.Seconds(), "verification_duration_sec")
	b.ReportMetric(float64(storedCount), "messages_stored")
}

// Benchmark Memory Usage
func BenchmarkMemoryUsage(b *testing.B) {
	configs := []*BenchmarkConfig{
		{MessageCount: 1000, MessageSize: 100},
		{MessageCount: 5000, MessageSize: 500},
		{MessageCount: 10000, MessageSize: 1000},
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("Memory_Messages%d_Size%d", config.MessageCount, config.MessageSize), func(b *testing.B) {
			var m1, m2 runtime.MemStats

			// Measure initial memory
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				storage := NewMockMessageStore()
				ctx := context.Background()

				for j := 0; j < config.MessageCount; j++ {
					message := &types.PortaskMessage{
						ID:        types.MessageID(fmt.Sprintf("mem-test-msg-%d", j)),
						Topic:     types.TopicName("memory.test"),
						Payload:   generatePayload(config.MessageSize),
						Timestamp: time.Now().UnixNano(),
					}
					storage.Store(ctx, message)
				}
			}

			b.StopTimer()

			// Measure final memory
			runtime.GC()
			runtime.ReadMemStats(&m2)

			memoryUsed := m2.Alloc - m1.Alloc
			b.ReportMetric(float64(memoryUsed)/1024/1024, "memory_mb")
		})
	}
}

// Benchmark Throughput Test
func BenchmarkThroughputTest(b *testing.B) {
	// Check Dragonfly availability
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 5})
	ctx := context.Background()
	if err := testClient.Ping(ctx).Err(); err != nil {
		b.Skipf("Dragonfly not available: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	durations := []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second}
	userCounts := []int{5, 10, 20}

	for _, duration := range durations {
		for _, userCount := range userCounts {
			b.Run(fmt.Sprintf("Throughput_%ds_%dusers", int(duration.Seconds()), userCount), func(b *testing.B) {

				// Create Dragonfly storage
				storageConfig := &storage.DragonflyStorageConfig{
					Addr: "localhost:6379", Password: "", DB: 5,
					PoolSize: userCount + 10, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
				}

				dragonflyStorage, err := storage.NewDragonflyStorage(storageConfig)
				require.NoError(b, err)
				defer dragonflyStorage.Close()

				codecManager := &serialization.CodecManager{}
				protocolHandler := NewJSONCompatibleProtocolHandler(codecManager, dragonflyStorage)

				serverConfig := &ServerConfig{
					Address: "127.0.0.1", Port: 0, MaxConnections: userCount + 10,
					ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second,
					Network: "tcp", ReadBufferSize: 8192, WriteBufferSize: 8192,
					WorkerPoolSize: userCount + 5,
				}

				server := NewTCPServer(serverConfig, protocolHandler)
				serverCtx, cancel := context.WithTimeout(context.Background(), duration+30*time.Second)
				defer cancel()

				go func() { server.Start(serverCtx) }()
				time.Sleep(300 * time.Millisecond)

				addr := server.GetAddr()
				require.NotNil(b, addr, "Server should start successfully")

				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					// Clear database
					testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 5})
					testClient.FlushDB(ctx)
					testClient.Close()

					totalMessages := runThroughputTest(b, addr.String(), userCount, duration)
					throughput := float64(totalMessages) / duration.Seconds()
					b.ReportMetric(throughput, "msg/sec")
					b.ReportMetric(float64(totalMessages), "total_messages")
				}

				server.Stop(serverCtx)
			})
		}
	}
}

// Run throughput test
func runThroughputTest(b *testing.B, serverAddr string, userCount int, duration time.Duration) int64 {
	var wg sync.WaitGroup
	var totalMessages int64
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	for userID := 0; userID < userCount; userID++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()

			conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
			if err != nil {
				return
			}
			defer conn.Close()

			messageCounter := 0
			ticker := time.NewTicker(10 * time.Millisecond) // Send message every 10ms
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					message := map[string]interface{}{
						"type":  "publish",
						"topic": fmt.Sprintf("throughput.user%d", uid),
						"data":  fmt.Sprintf("Message %d from user %d", messageCounter, uid),
					}

					jsonBytes, err := json.Marshal(message)
					if err != nil {
						continue
					}

					_, err = conn.Write(append(jsonBytes, '\n'))
					if err != nil {
						return // Connection lost
					}

					atomic.AddInt64(&totalMessages, 1)
					messageCounter++
				}
			}
		}(userID)
	}

	wg.Wait()
	return atomic.LoadInt64(&totalMessages)
}

// Helper function to generate payload of specified size
func generatePayload(size int) []byte {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte('A' + (i % 26))
	}
	return payload
}

// Test function to run comprehensive benchmark suite
func TestRunBenchmarkSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark suite in short mode")
	}

	// Check Dragonfly availability
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 6})
	ctx := context.Background()
	if err := testClient.Ping(ctx).Err(); err != nil {
		t.Skipf("Dragonfly not available for benchmark suite: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	t.Log("🚀 Starting Comprehensive Benchmark Suite")

	// Run quick performance comparison
	t.Run("Quick Performance Comparison", func(t *testing.T) {
		config := &BenchmarkConfig{
			MessageCount:    1000,
			ConcurrentUsers: 10,
			MessageSize:     256,
			TopicCount:      3,
		}

		// Test Mock Storage
		mockStorage := NewMockMessageStore()
		mockStart := time.Now()
		runStorageBenchmarkTest(t, mockStorage, config)
		mockDuration := time.Since(mockStart)

		// Test Dragonfly Storage
		storageConfig := &storage.DragonflyStorageConfig{
			Addr: "localhost:6379", Password: "", DB: 6,
			PoolSize: 20, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
		}

		dragonflyStorage, err := storage.NewDragonflyStorage(storageConfig)
		require.NoError(t, err)
		defer dragonflyStorage.Close()

		dragonflyStart := time.Now()
		runStorageBenchmarkTest(t, dragonflyStorage, config)
		dragonflyDuration := time.Since(dragonflyStart)

		// Report comparison
		mockThroughput := float64(config.MessageCount) / mockDuration.Seconds()
		dragonflyThroughput := float64(config.MessageCount) / dragonflyDuration.Seconds()

		t.Logf("📊 PERFORMANCE COMPARISON:")
		t.Logf("🏃 Mock Storage: %.2f msg/sec (%.2fs total)", mockThroughput, mockDuration.Seconds())
		t.Logf("🐉 Dragonfly Storage: %.2f msg/sec (%.2fs total)", dragonflyThroughput, dragonflyDuration.Seconds())
		t.Logf("📈 Performance Ratio: %.2fx", mockThroughput/dragonflyThroughput)
	})

	t.Log("✅ Benchmark Suite Completed")
}

// Helper function for test benchmarks
func runStorageBenchmarkTest(t *testing.T, store storage.MessageStore, config *BenchmarkConfig) {
	ctx := context.Background()
	var wg sync.WaitGroup

	messagesPerUser := config.MessageCount / config.ConcurrentUsers

	for userID := 0; userID < config.ConcurrentUsers; userID++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()

			for i := 0; i < messagesPerUser; i++ {
				message := &types.PortaskMessage{
					ID:        types.MessageID(fmt.Sprintf("test-user%d-msg%d", uid, i)),
					Topic:     types.TopicName(fmt.Sprintf("test.topic%d", uid%config.TopicCount)),
					Payload:   generatePayload(config.MessageSize),
					Timestamp: time.Now().UnixNano(),
				}

				if err := store.Store(ctx, message); err != nil {
					t.Errorf("Store failed: %v", err)
				}
			}
		}(userID)
	}

	wg.Wait()
}

// OPTIMIZED BENCHMARKS FOR 1M MSG/MIN TARGET

// Benchmark Optimized Dragonfly Storage Performance (Target: 1M msg/min = 16,666 msg/sec)
func BenchmarkOptimizedDragonflyStoragePerformance(b *testing.B) {
	// Check Dragonfly availability
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 4})
	ctx := context.Background()
	if err := testClient.Ping(ctx).Err(); err != nil {
		b.Skipf("Dragonfly not available: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	configs := []*BenchmarkConfig{
		{MessageCount: 1000, ConcurrentUsers: 50, MessageSize: 64, TopicCount: 10, BatchSize: 100},
		{MessageCount: 5000, ConcurrentUsers: 100, MessageSize: 128, TopicCount: 20, BatchSize: 200},
		{MessageCount: 10000, ConcurrentUsers: 200, MessageSize: 256, TopicCount: 50, BatchSize: 500},
		{MessageCount: 20000, ConcurrentUsers: 400, MessageSize: 512, TopicCount: 100, BatchSize: 1000},
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("OptimizedDragonfly_Messages%d_Users%d_Size%d_Batch%d",
			config.MessageCount, config.ConcurrentUsers, config.MessageSize, config.BatchSize), func(b *testing.B) {
			// Create optimized Dragonfly storage
			storageConfig := &storage.DragonflyStorageConfig{
				Addr: "localhost:6379", Password: "", DB: 4,
				PoolSize:     100,             // Increased pool size
				ReadTimeout:  1 * time.Second, // Faster timeouts
				WriteTimeout: 1 * time.Second,
			}

			dragonflyStorage, err := storage.NewDragonflyStorage(storageConfig)
			require.NoError(b, err)
			defer dragonflyStorage.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Clear database for each iteration
				testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 4})
				testClient.FlushDB(ctx)
				testClient.Close()

				runOptimizedStorageBenchmark(b, dragonflyStorage, config)
			}
		})
	}
}

// Optimized storage benchmark with batching and pipelining
func runOptimizedStorageBenchmark(b *testing.B, store storage.MessageStore, config *BenchmarkConfig) {
	ctx := context.Background()

	// Safety checks
	if config.ConcurrentUsers <= 0 {
		b.Fatalf("ConcurrentUsers must be greater than 0, got: %d", config.ConcurrentUsers)
	}
	if config.TopicCount <= 0 {
		b.Fatalf("TopicCount must be greater than 0, got: %d", config.TopicCount)
	}

	var wg sync.WaitGroup
	messagesPerUser := config.MessageCount / config.ConcurrentUsers
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 100 // Default batch size
	}

	startTime := time.Now()

	// Channel for batching messages
	messageChan := make(chan *types.PortaskMessage, batchSize*10)

	// Start batch processor goroutines
	numProcessors := runtime.NumCPU()
	for p := 0; p < numProcessors; p++ {
		go func() {
			batch := make([]*types.PortaskMessage, 0, batchSize)
			ticker := time.NewTicker(1 * time.Millisecond) // Micro-batching
			defer ticker.Stop()

			for {
				select {
				case msg, ok := <-messageChan:
					if !ok {
						// Channel closed, process remaining batch
						if len(batch) > 0 {
							processBatch(ctx, store, batch)
						}
						return
					}
					batch = append(batch, msg)
					if len(batch) >= batchSize {
						processBatch(ctx, store, batch)
						batch = batch[:0] // Reset slice
					}
				case <-ticker.C:
					if len(batch) > 0 {
						processBatch(ctx, store, batch)
						batch = batch[:0]
					}
				}
			}
		}()
	}

	// Message generators
	for userID := 0; userID < config.ConcurrentUsers; userID++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()

			// Pre-allocate payloads for better performance
			payload := generatePayload(config.MessageSize)

			for i := 0; i < messagesPerUser; i++ {
				message := &types.PortaskMessage{
					ID:        types.MessageID(fmt.Sprintf("opt-user%d-msg%d", uid, i)),
					Topic:     types.TopicName(fmt.Sprintf("opt.topic%d", uid%config.TopicCount)),
					Payload:   payload, // Reuse payload
					Timestamp: time.Now().UnixNano(),
				}

				select {
				case messageChan <- message:
				case <-ctx.Done():
					return
				}
			}
		}(userID)
	}

	wg.Wait()
	close(messageChan)

	// Wait a bit for all batches to be processed
	time.Sleep(100 * time.Millisecond)

	duration := time.Since(startTime)

	// Report custom metrics
	messagesPerSecond := float64(config.MessageCount) / duration.Seconds()
	b.ReportMetric(messagesPerSecond, "msg/sec")
	b.ReportMetric(duration.Seconds(), "duration_sec")
}

// Process a batch of messages
func processBatch(ctx context.Context, store storage.MessageStore, batch []*types.PortaskMessage) {
	// Create MessageBatch and use StoreBatch
	messageBatch := &types.MessageBatch{
		Messages:  batch,
		BatchID:   fmt.Sprintf("batch-%d", time.Now().UnixNano()),
		CreatedAt: time.Now().UnixNano(),
	}

	if err := store.StoreBatch(ctx, messageBatch); err != nil {
		// Fallback to individual stores with goroutines
		var wg sync.WaitGroup
		for _, msg := range batch {
			wg.Add(1)
			go func(m *types.PortaskMessage) {
				defer wg.Done()
				store.Store(ctx, m)
			}(msg)
		}
		wg.Wait()
	}
}

// Benchmark Pipeline Performance
func BenchmarkPipelineDragonflyPerformance(b *testing.B) {
	// Check Dragonfly availability
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 5})
	ctx := context.Background()
	if err := testClient.Ping(ctx).Err(); err != nil {
		b.Skipf("Dragonfly not available: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	b.Run("Pipeline_10K_Messages", func(b *testing.B) {
		// Create optimized client for pipelining
		client := redis.NewClient(&redis.Options{
			Addr:         "localhost:6379",
			DB:           5,
			PoolSize:     50,
			MinIdleConns: 10,
			DialTimeout:  100 * time.Millisecond,
			ReadTimeout:  500 * time.Millisecond,
			WriteTimeout: 500 * time.Millisecond,
		})
		defer client.Close()

		const messageCount = 10000
		const pipelineSize = 1000

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			client.FlushDB(ctx)

			startTime := time.Now()

			// Use pipeline for batch operations
			for batch := 0; batch < messageCount; batch += pipelineSize {
				pipe := client.Pipeline()

				end := batch + pipelineSize
				if end > messageCount {
					end = messageCount
				}

				for j := batch; j < end; j++ {
					streamKey := fmt.Sprintf("portask:topic:bench.topic%d", j%10)
					pipe.XAdd(ctx, &redis.XAddArgs{
						Stream: streamKey,
						Values: map[string]interface{}{
							"id":        fmt.Sprintf("pipe-msg-%d", j),
							"payload":   generatePayload(256),
							"timestamp": time.Now().UnixNano(),
						},
					})
				}

				_, err := pipe.Exec(ctx)
				if err != nil {
					b.Errorf("Pipeline exec failed: %v", err)
				}
			}

			duration := time.Since(startTime)
			messagesPerSecond := float64(messageCount) / duration.Seconds()

			b.ReportMetric(messagesPerSecond, "msg/sec")
			b.ReportMetric(duration.Seconds(), "duration_sec")
		}
	})
}

// Benchmark Memory-Optimized Performance
func BenchmarkMemoryOptimizedDragonflyPerformance(b *testing.B) {
	// Check Dragonfly availability
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 6})
	ctx := context.Background()
	if err := testClient.Ping(ctx).Err(); err != nil {
		b.Skipf("Dragonfly not available: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	b.Run("MemoryOptimized_5K_Messages", func(b *testing.B) {
		client := redis.NewClient(&redis.Options{
			Addr:         "localhost:6379",
			DB:           6,
			PoolSize:     20,
			MinIdleConns: 5,
		})
		defer client.Close()

		const messageCount = 5000

		// Pre-allocate message pool
		messagePool := &sync.Pool{
			New: func() interface{} {
				return &types.PortaskMessage{}
			},
		}

		// Pre-allocate payload pool
		payloadPool := &sync.Pool{
			New: func() interface{} {
				return generatePayload(256)
			},
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			client.FlushDB(ctx)

			startTime := time.Now()

			var wg sync.WaitGroup
			semaphore := make(chan struct{}, 100) // Limit concurrent operations

			for j := 0; j < messageCount; j++ {
				wg.Add(1)
				semaphore <- struct{}{}

				go func(msgID int) {
					defer wg.Done()
					defer func() { <-semaphore }()

					// Get message from pool
					msg := messagePool.Get().(*types.PortaskMessage)
					payload := payloadPool.Get().([]byte)

					// Configure message
					msg.ID = types.MessageID(fmt.Sprintf("mem-msg-%d", msgID))
					msg.Topic = types.TopicName(fmt.Sprintf("mem.topic%d", msgID%5))
					msg.Payload = payload
					msg.Timestamp = time.Now().UnixNano()

					// Store to Redis
					streamKey := fmt.Sprintf("portask:topic:%s", msg.Topic)
					client.XAdd(ctx, &redis.XAddArgs{
						Stream: streamKey,
						Values: map[string]interface{}{
							"id":        string(msg.ID),
							"payload":   msg.Payload,
							"timestamp": msg.Timestamp,
						},
					})

					// Return to pool
					messagePool.Put(msg)
					payloadPool.Put(payload)
				}(j)
			}

			wg.Wait()

			duration := time.Since(startTime)
			messagesPerSecond := float64(messageCount) / duration.Seconds()

			b.ReportMetric(messagesPerSecond, "msg/sec")
			b.ReportMetric(duration.Seconds(), "duration_sec")
		}
	})
}

// Benchmark Async Fire-and-Forget Performance
func BenchmarkAsyncDragonflyPerformance(b *testing.B) {
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 7})
	ctx := context.Background()
	if err := testClient.Ping(ctx).Err(); err != nil {
		b.Skipf("Dragonfly not available: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	b.Run("Async_20K_Messages", func(b *testing.B) {
		client := redis.NewClient(&redis.Options{
			Addr:         "localhost:6379",
			DB:           7,
			PoolSize:     200,
			MinIdleConns: 50,
			DialTimeout:  50 * time.Millisecond,
			ReadTimeout:  200 * time.Millisecond,
			WriteTimeout: 200 * time.Millisecond,
		})
		defer client.Close()

		const messageCount = 20000

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			client.FlushDB(ctx)

			startTime := time.Now()

			// Fire-and-forget with channels
			workChan := make(chan int, messageCount)

			// Start workers
			numWorkers := runtime.NumCPU() * 4
			var wg sync.WaitGroup

			for w := 0; w < numWorkers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for msgID := range workChan {
						streamKey := fmt.Sprintf("portask:topic:async.topic%d", msgID%20)
						client.XAdd(ctx, &redis.XAddArgs{
							Stream: streamKey,
							Values: map[string]interface{}{
								"id":        fmt.Sprintf("async-msg-%d", msgID),
								"payload":   generatePayload(128),
								"timestamp": time.Now().UnixNano(),
							},
						})
					}
				}()
			}

			// Send work
			for j := 0; j < messageCount; j++ {
				workChan <- j
			}
			close(workChan)

			wg.Wait()

			duration := time.Since(startTime)
			messagesPerSecond := float64(messageCount) / duration.Seconds()

			b.ReportMetric(messagesPerSecond, "msg/sec")
			b.ReportMetric(duration.Seconds(), "duration_sec")
		}
	})
}

// BenchmarkExtremePerformanceDragonflyStorage - Push towards 10M+ msg/sec
func BenchmarkExtremePerformanceDragonflyStorage(b *testing.B) {
	// Check Dragonfly availability
	ctx := context.Background()
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 10})
	err := testClient.Ping(ctx).Err()
	if err != nil {
		b.Skipf("Dragonfly not available: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	extremeConfigs := []BenchmarkConfig{
		// Extreme batching with max concurrency
		{
			MessageCount:       50000,
			ConcurrentUsers:    800,  // Max concurrency
			MessageSize:        128,  // Optimal size
			TopicCount:         3,    // Reduce topic overhead
			BatchSize:          2000, // Large batches
			TestDuration:       10 * time.Second,
			ConnectionPoolSize: 100,
		},
		// Ultra-high volume
		{
			MessageCount:       100000,
			ConcurrentUsers:    1000, // Ultimate concurrency
			MessageSize:        64,   // Minimal size
			TopicCount:         2,    // Minimal topics
			BatchSize:          5000, // Massive batches
			TestDuration:       15 * time.Second,
			ConnectionPoolSize: 150,
		},
		// Maximum throughput configuration
		{
			MessageCount:       200000,
			ConcurrentUsers:    1200,  // Beyond typical limits
			MessageSize:        32,    // Ultra-small messages
			TopicCount:         1,     // Single topic
			BatchSize:          10000, // Extreme batching
			TestDuration:       20 * time.Second,
			ConnectionPoolSize: 200,
		},
	}

	for i, config := range extremeConfigs {
		name := fmt.Sprintf("ExtremeDragonfly_Messages%d_Users%d_Size%d_Batch%d",
			config.MessageCount, config.ConcurrentUsers, config.MessageSize, config.BatchSize)

		b.Run(name, func(b *testing.B) {
			// Create optimized Dragonfly storage with extreme settings
			storageConfig := &storage.DragonflyStorageConfig{
				Addr:         "localhost:6379",
				Password:     "",
				DB:           10 + i, // Separate DB for each test
				PoolSize:     config.ConnectionPoolSize,
				ReadTimeout:  1 * time.Second, // Faster timeouts
				WriteTimeout: 1 * time.Second,
			}

			dragonflyStorage, err := storage.NewDragonflyStorage(storageConfig)
			require.NoError(b, err)
			defer dragonflyStorage.Close()

			// Pre-warm the connection pool
			ctx := context.Background()
			for warmup := 0; warmup < 10; warmup++ {
				testMsg := &types.PortaskMessage{
					ID:        types.MessageID(fmt.Sprintf("warmup-%d", warmup)),
					Topic:     types.TopicName("warmup"),
					Payload:   []byte("warmup"),
					Timestamp: time.Now().UnixNano(),
				}
				dragonflyStorage.Store(ctx, testMsg)
			}

			b.ResetTimer()
			result := runExtremeOptimizedBenchmark(b, dragonflyStorage, &config)

			// Calculate and report metrics
			durationSec := result.Duration.Seconds()
			msgPerSec := float64(result.TotalMessages) / durationSec
			msgPerMin := msgPerSec * 60

			b.ReportMetric(durationSec, "duration_sec")
			b.ReportMetric(msgPerSec, "msg/sec")
			b.ReportMetric(msgPerMin, "msg/min")
			b.ReportMetric(float64(result.TotalErrors), "errors")

			b.Logf("🚀 EXTREME PERFORMANCE: %.0f msg/sec (%.1fM msg/min), Errors: %d",
				msgPerSec, msgPerMin/1000000, result.TotalErrors)
		})
	}
}

// runExtremeOptimizedBenchmark implements extreme optimization strategies
func runExtremeOptimizedBenchmark(b *testing.B, storage storage.MessageStore, config *BenchmarkConfig) *BenchmarkResult {
	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration+5*time.Second)
	defer cancel()

	var totalMessages int64
	var totalErrors int64
	var wg sync.WaitGroup

	// Pre-allocate all messages using object pooling
	messagePool := &sync.Pool{
		New: func() interface{} {
			return &types.PortaskMessage{}
		},
	}

	// Pre-generate all message data to avoid allocation during benchmark
	messageData := make([][]byte, config.MessageSize)
	for i := range messageData {
		messageData[i] = make([]byte, config.MessageSize)
		for j := range messageData[i] {
			messageData[i][j] = byte('A' + (i+j)%26)
		}
	}

	// Topic pool for minimal allocation
	topics := make([]types.TopicName, config.TopicCount)
	for i := range topics {
		topics[i] = types.TopicName(fmt.Sprintf("extreme.topic.%d", i))
	}

	startTime := time.Now()

	// Ultra-high concurrency workers
	for userID := 0; userID < config.ConcurrentUsers; userID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Local batch for each worker
			batch := make([]*types.PortaskMessage, 0, config.BatchSize)
			messagesPerUser := config.MessageCount / config.ConcurrentUsers

			for msgIndex := 0; msgIndex < messagesPerUser; msgIndex++ {
				// Reuse message from pool
				msg := messagePool.Get().(*types.PortaskMessage)

				// Set message data with minimal allocation
				msg.ID = types.MessageID(fmt.Sprintf("extreme-%d-%d", id, msgIndex))
				msg.Topic = topics[msgIndex%len(topics)]
				msg.Payload = messageData[msgIndex%len(messageData)]
				msg.Timestamp = time.Now().UnixNano()

				batch = append(batch, msg)

				// Process batch when full or last message
				if len(batch) >= config.BatchSize || msgIndex == messagesPerUser-1 {
					if err := processExtremeBatch(ctx, storage, batch); err != nil {
						atomic.AddInt64(&totalErrors, int64(len(batch)))
					} else {
						atomic.AddInt64(&totalMessages, int64(len(batch)))
					}

					// Return messages to pool and reset batch
					for _, poolMsg := range batch {
						messagePool.Put(poolMsg)
					}
					batch = batch[:0] // Reuse slice
				}

				// Yield to prevent CPU hogging
				if msgIndex%1000 == 0 {
					runtime.Gosched()
				}
			}
		}(userID)
	}

	wg.Wait()
	duration := time.Since(startTime)

	return &BenchmarkResult{
		TotalMessages:     totalMessages,
		TotalErrors:       totalErrors,
		Duration:          duration,
		MessagesPerSecond: float64(totalMessages) / duration.Seconds(),
	}
}

// processExtremeBatch handles batch processing with maximum efficiency
func processExtremeBatch(ctx context.Context, storage storage.MessageStore, batch []*types.PortaskMessage) error {
	// Use batch operations available in MessageStore interface
	messageBatch := &types.MessageBatch{
		Messages: make([]*types.PortaskMessage, len(batch)),
	}
	copy(messageBatch.Messages, batch)

	// StoreBatch is available in MessageStore interface
	return storage.StoreBatch(ctx, messageBatch)
}

// BenchmarkMemoryFootprintOptimization - Test memory usage optimization
func BenchmarkMemoryFootprintOptimization(b *testing.B) {
	ctx := context.Background()
	testClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	err := testClient.Ping(ctx).Err()
	if err != nil {
		b.Skipf("Dragonfly not available: %v", err)
	}
	testClient.FlushDB(ctx)
	testClient.Close()

	config := &BenchmarkConfig{
		MessageCount:       50000,
		ConcurrentUsers:    500,
		MessageSize:        64,
		TopicCount:         2,
		BatchSize:          1000,
		TestDuration:       10 * time.Second,
		ConnectionPoolSize: 50,
	}

	b.Run("MemoryOptimized", func(b *testing.B) {
		storageConfig := &storage.DragonflyStorageConfig{
			Addr:         "localhost:6379",
			Password:     "",
			DB:           15,
			PoolSize:     config.ConnectionPoolSize,
			ReadTimeout:  2 * time.Second,
			WriteTimeout: 2 * time.Second,
		}

		dragonflyStorage, err := storage.NewDragonflyStorage(storageConfig)
		require.NoError(b, err)
		defer dragonflyStorage.Close()

		// Force garbage collection before test
		runtime.GC()

		var memBefore, memAfter runtime.MemStats
		runtime.ReadMemStats(&memBefore)

		b.ResetTimer()
		result := runExtremeOptimizedBenchmark(b, dragonflyStorage, config)

		runtime.ReadMemStats(&memAfter)

		durationSec := result.Duration.Seconds()
		msgPerSec := float64(result.TotalMessages) / durationSec
		memUsedMB := float64(memAfter.Alloc-memBefore.Alloc) / 1024 / 1024

		b.ReportMetric(durationSec, "duration_sec")
		b.ReportMetric(msgPerSec, "msg/sec")
		b.ReportMetric(memUsedMB, "memory_mb")
		b.ReportMetric(float64(result.TotalErrors), "errors")

		b.Logf("🧠 MEMORY OPTIMIZED: %.0f msg/sec, Memory used: %.2f MB, Errors: %d",
			msgPerSec, memUsedMB, result.TotalErrors)
	})
}
