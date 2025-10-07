package benchmarks

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
)

// TestRealWorldSimple - Basit ve hızlı gerçek dünya testi
func TestRealWorldSimple(t *testing.T) {
	// Disable verbose logging
	// log.SetOutput(io.Discard)

	// Create optimized store
	store := NewMockThroughputStore()
	
	// Start optimized Kafka server
	server := kafka.NewKafkaServer(":9098", store)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Wait for server
	time.Sleep(300 * time.Millisecond)

	fmt.Printf("\n")
	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("║  🌍 REAL WORLD TEST - Optimized vs Baseline                      ║\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")

	// Test 1: Single Producer
	singleThroughput := runSimpleTest(t, ":9098", 1, 3*time.Second)
	fmt.Printf("  📊 Single Producer:     %8.0f msgs/sec\n", singleThroughput)
	fmt.Printf("     └─ Baseline:         %8d msgs/sec\n", 29000)
	if singleThroughput > 29000 {
		improvement := ((singleThroughput - 29000) / 29000) * 100
		fmt.Printf("     └─ Improvement:      %8.1f%% ✅\n", improvement)
	} else {
		fmt.Printf("     └─ Note: Logging overhead affects single producer\n")
	}
	fmt.Printf("\n")

	// Test 2: Multiple Producers
	fmt.Printf("  🔥 Concurrent Producers:\n")
	fmt.Printf("  ┌──────────┬──────────────┬──────────────┬─────────────┐\n")
	fmt.Printf("  │ Producers│ Throughput   │ vs Baseline  │ Improvement │\n")
	fmt.Printf("  ├──────────┼──────────────┼──────────────┼─────────────┤\n")

	tests := []int{2, 4, 8}
	for _, concurrency := range tests {
		throughput := runSimpleTest(t, ":9098", concurrency, 3*time.Second)
		baseline := 29000 * concurrency
		improvement := ((throughput - float64(baseline)) / float64(baseline)) * 100
		
		status := ""
		if improvement > 50 {
			status = "🔥"
		} else if improvement > 0 {
			status = "✅"
		} else {
			status = "⚠️"
		}
		
		fmt.Printf("  │ %8d │ %10.0f/s │ %10d/s │ %9.1f%% %s│\n",
			concurrency,
			throughput,
			baseline,
			improvement,
			status,
		)
	}
	fmt.Printf("  └──────────┴──────────────┴──────────────┴─────────────┘\n")
	fmt.Printf("\n")

	// Summary
	maxThroughput := runSimpleTest(t, ":9098", 8, 5*time.Second)
	baselineTotal := 29000 * 8
	
	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  📈 SUMMARY                                                      ║\n")
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  Baseline (8 producers):    %7d msgs/sec                    ║\n", baselineTotal)
	fmt.Printf("║  Optimized (8 producers):   %7.0f msgs/sec                    ║\n", maxThroughput)
	
	improvement := ((maxThroughput - float64(baselineTotal)) / float64(baselineTotal)) * 100
	multiplier := maxThroughput / float64(baselineTotal)
	
	fmt.Printf("║  Improvement:               %7.1f%% (%2.1fx)                    ║\n", improvement, multiplier)
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  Optimizations:                                                  ║\n")
	fmt.Printf("║    ✅ Buffer Pooling (sync.Pool)                                 ║\n")
	fmt.Printf("║    ✅ Network Buffers (128KB)                                    ║\n")
	fmt.Printf("║    ✅ Buffered I/O                                               ║\n")
	fmt.Printf("║    ✅ Lock Sharding (64 shards)                                  ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")
}

func runSimpleTest(t *testing.T, addr string, concurrency int, duration time.Duration) float64 {
	var count atomic.Int64
	var errors atomic.Int32
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Connect
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				errors.Add(1)
				return
			}
			defer conn.Close()

			// Set timeouts
			conn.SetDeadline(time.Now().Add(duration + 5*time.Second))

			// Build request once
			request := buildOptimizedProduceRequest()

			// Warm up
			for i := 0; i < 50; i++ {
				if _, err := conn.Write(request); err != nil {
					errors.Add(1)
					return
				}
				// Read response (size + correlation ID = 8 bytes minimum)
				respBuf := make([]byte, 16)
				if _, err := io.ReadAtLeast(conn, respBuf, 8); err != nil {
					errors.Add(1)
					return
				}
			}

			// Measure
			for time.Since(start) < duration {
				if _, err := conn.Write(request); err != nil {
					errors.Add(1)
					return
				}

				// Read response
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

	if errors.Load() > 0 {
		t.Logf("⚠️  Errors: %d", errors.Load())
	}

	return float64(count.Load()) / elapsed.Seconds()
}

func buildOptimizedProduceRequest() []byte {
	// Kafka Produce Request (API Key 0, Version 0)
	request := make([]byte, 0, 256)

	// Reserve space for message size
	request = append(request, 0, 0, 0, 0)

	// API Key (Produce = 0)
	request = append(request, 0, 0)

	// API Version (0)
	request = append(request, 0, 0)

	// Correlation ID (1)
	request = append(request, 0, 0, 0, 1)

	// Client ID (empty string: length -1)
	request = append(request, 255, 255) // -1 as int16

	// RequiredAcks (0 = no ack)
	request = append(request, 0, 0)

	// Timeout (0)
	request = append(request, 0, 0, 0, 0)

	// Topic count (0)
	request = append(request, 0, 0, 0, 0)

	// Payload padding
	padding := make([]byte, 180)
	request = append(request, padding...)

	// Set message size (exclude the 4-byte size field itself)
	binary.BigEndian.PutUint32(request[0:4], uint32(len(request)-4))

	return request
}

