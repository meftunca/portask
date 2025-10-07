package benchmarks

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
)

// TestPhase1OptimizedThroughput tests throughput after Phase 1 optimizations
func TestPhase1OptimizedThroughput(t *testing.T) {
	// Create mock store for testing
	store := NewMockThroughputStore()

	// Create Kafka server with optimizations
	server := kafka.NewKafkaServer(":9095", store)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	t.Run("ProduceThroughput_Optimized", func(t *testing.T) {
		testOptimizedProduceThroughput(t, ":9095")
	})

	t.Run("ConcurrentProducers_Optimized", func(t *testing.T) {
		testOptimizedConcurrentProducers(t, ":9095")
	})
}

func testOptimizedProduceThroughput(t *testing.T, addr string) {
	// Connect to server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create Kafka produce request (simplified)
	produceRequest := createKafkaProduceRequest("test-topic", 0, [][]byte{[]byte("test-message")})

	// Warm up
	for i := 0; i < 1000; i++ {
		if _, err := conn.Write(produceRequest); err != nil {
			t.Fatalf("Warm-up failed: %v", err)
		}
		// Read response
		sizeBuf := make([]byte, 4)
		conn.Read(sizeBuf)
	}

	// Benchmark
	duration := 5 * time.Second
	var count atomic.Int64
	start := time.Now()
	done := make(chan struct{})

	go func() {
		defer close(done)
		for time.Since(start) < duration {
			if _, err := conn.Write(produceRequest); err != nil {
				t.Errorf("Write error: %v", err)
				return
			}
			// Read response (simplified)
			sizeBuf := make([]byte, 4)
			conn.Read(sizeBuf)
			count.Add(1)
		}
	}()

	<-done
	elapsed := time.Since(start)

	total := count.Load()
	throughput := float64(total) / elapsed.Seconds()

	fmt.Printf("\n╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  📊 PHASE 1 OPTIMIZATION RESULTS (Single Producer)              ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("  Total Messages:     %d\n", total)
	fmt.Printf("  Duration:           %.2fs\n", elapsed.Seconds())
	fmt.Printf("  Throughput:         %.0f msgs/sec\n", throughput)
	fmt.Printf("  Avg Latency:        %.2fµs\n", (elapsed.Seconds()/float64(total))*1000000)
	fmt.Printf("\n")

	// Expected improvement: 40-70% from buffer pooling + buffered I/O
	// Baseline: ~29K msgs/sec
	// Target:   ~40-50K msgs/sec
	if throughput < 35000 {
		t.Logf("⚠️  Warning: Throughput below target (%.0f < 35K msgs/sec)", throughput)
	} else {
		t.Logf("✅ Throughput meets target: %.0f msgs/sec", throughput)
	}
}

func testOptimizedConcurrentProducers(t *testing.T, addr string) {
	concurrencyLevels := []int{1, 2, 4, 8, 16}

	fmt.Printf("\n╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  📊 PHASE 1 OPTIMIZATION - CONCURRENT PRODUCERS                  ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n\n")

	for _, concurrency := range concurrencyLevels {
		throughput := runConcurrentProducers(t, addr, concurrency, 3*time.Second)

		fmt.Printf("  Concurrency %2d:  %7.0f msgs/sec\n", concurrency, throughput)

		// Expected scaling with optimizations
		expectedMin := float64(30000 * concurrency) * 0.7 // 70% linear scaling
		if throughput < expectedMin {
			t.Logf("⚠️  Warning: Sub-linear scaling at concurrency %d", concurrency)
		}
	}

	fmt.Printf("\n")
}

func runConcurrentProducers(t *testing.T, addr string, concurrency int, duration time.Duration) float64 {
	var count atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Errorf("Failed to connect: %v", err)
				return
			}
			defer conn.Close()

			produceRequest := createKafkaProduceRequest("test-topic", 0, [][]byte{[]byte("test-message")})

			for time.Since(start) < duration {
				if _, err := conn.Write(produceRequest); err != nil {
					return
				}
				// Read response
				sizeBuf := make([]byte, 4)
				conn.Read(sizeBuf)
				count.Add(1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	return float64(count.Load()) / elapsed.Seconds()
}

func createKafkaProduceRequest(topic string, partition int32, messages [][]byte) []byte {
	// Simplified Kafka Produce request (API Key 0)
	// Format: [message_size][api_key][api_version][correlation_id][client_id][...payload]
	
	payload := make([]byte, 200) // 200 byte payload
	
	// Build request
	request := make([]byte, 0, 250)
	
	// Message size (will be set at the end)
	sizeBuf := make([]byte, 4)
	request = append(request, sizeBuf...)
	
	// API Key (Produce = 0)
	apiKeyBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(apiKeyBuf, 0)
	request = append(request, apiKeyBuf...)
	
	// API Version
	versionBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(versionBuf, 0)
	request = append(request, versionBuf...)
	
	// Correlation ID
	corrIDBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(corrIDBuf, 1)
	request = append(request, corrIDBuf...)
	
	// Client ID (empty)
	clientIDLen := make([]byte, 2)
	binary.BigEndian.PutUint16(clientIDLen, 0)
	request = append(request, clientIDLen...)
	
	// Payload
	request = append(request, payload...)
	
	// Set message size (excluding size field itself)
	binary.BigEndian.PutUint32(request[:4], uint32(len(request)-4))
	
	return request
}

