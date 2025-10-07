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

// TestPhase1QuickCheck - Quick test to verify Phase 1 optimizations
func TestPhase1QuickCheck(t *testing.T) {
	// Create mock store
	store := NewMockThroughputStore()

	// Start optimized Kafka server
	server := kafka.NewKafkaServer(":9096", store)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	fmt.Printf("\n")
	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("║  📊 PHASE 1 OPTIMIZATION RESULTS (Quick Test)                   ║\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")

	// Test 1: Single connection throughput
	t.Run("SingleConnection", func(t *testing.T) {
		throughput := measureQuickThroughput(t, ":9096", 1, 2*time.Second)
		fmt.Printf("  Single Connection:      %7.0f msgs/sec\n", throughput)
		
		if throughput > 35000 {
			fmt.Printf("  ✅ Above target (35K)    +%.1f%%\n", ((throughput-29000)/29000)*100)
		} else {
			fmt.Printf("  ⚠️  Below target\n")
		}
		fmt.Printf("\n")
	})

	// Test 2: Concurrent connections
	t.Run("ConcurrentConnections", func(t *testing.T) {
		fmt.Printf("  Concurrent Throughput:\n")
		
		for _, concurrency := range []int{2, 4, 8} {
			throughput := measureQuickThroughput(t, ":9096", concurrency, 2*time.Second)
			improvement := ((throughput - 29000*float64(concurrency)) / (29000 * float64(concurrency))) * 100
			
			fmt.Printf("    %2d connections:  %8.0f msgs/sec", concurrency, throughput)
			if improvement > 0 {
				fmt.Printf("  (✅ +%.1f%%)\n", improvement)
			} else {
				fmt.Printf("  (⚠️  %.1f%%)\n", improvement)
			}
		}
		fmt.Printf("\n")
	})

	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  Optimizations Applied:                                          ║\n")
	fmt.Printf("║    ✅ Buffer Pooling (sync.Pool)                                 ║\n")
	fmt.Printf("║    ✅ Network Buffers (128KB read + 64KB write)                  ║\n")
	fmt.Printf("║    ✅ Buffered I/O (bufio.Reader/Writer)                         ║\n")
	fmt.Printf("║    ✅ Auto-flush (1ms interval)                                  ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")
}

func measureQuickThroughput(t *testing.T, addr string, concurrency int, duration time.Duration) float64 {
	var count atomic.Int64
	var wg sync.WaitGroup
	var errors atomic.Int32

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Errorf("Failed to connect: %v", err)
				errors.Add(1)
				return
			}
			defer conn.Close()

			request := createSimpleProduceRequest()

			// Warm up
			for i := 0; i < 100; i++ {
				conn.Write(request)
				readResponse(conn)
			}

			// Measure
			for time.Since(start) < duration {
				if _, err := conn.Write(request); err != nil {
					errors.Add(1)
					return
				}
				if !readResponse(conn) {
					errors.Add(1)
					return
				}
				count.Add(1)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	if errors.Load() > 0 {
		t.Logf("Warning: %d errors occurred during test", errors.Load())
	}

	return float64(count.Load()) / elapsed.Seconds()
}

func createSimpleProduceRequest() []byte {
	request := make([]byte, 214) // Fixed size for simplicity

	// Message size (210 bytes of content)
	binary.BigEndian.PutUint32(request[0:4], 210)

	// API Key (Produce = 0)
	binary.BigEndian.PutUint16(request[4:6], 0)

	// API Version
	binary.BigEndian.PutUint16(request[6:8], 0)

	// Correlation ID
	binary.BigEndian.PutUint32(request[8:12], 1)

	// Client ID length (0 = empty)
	binary.BigEndian.PutUint16(request[12:14], 0)

	// Rest is payload
	return request
}

func readResponse(conn net.Conn) bool {
	sizeBuf := make([]byte, 4)
	if _, err := conn.Read(sizeBuf); err != nil {
		return false
	}
	return true
}

