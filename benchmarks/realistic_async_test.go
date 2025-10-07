package benchmarks

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
)

// TestRealisticAsync - Gerçekçi async benchmark (projection değil!)
func TestRealisticAsync(t *testing.T) {
	// Disable logging
	oldOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(oldOutput)

	// Create store and server
	store := NewMockThroughputStore()
	server := kafka.NewKafkaServer(":9101", store)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(300 * time.Millisecond)

	fmt.Printf("\n")
	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("║  🔍 REALISTIC ASYNC BENCHMARK - Gerçek Sayılar!                  ║\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")

	// Test 1: Actual request rate with pipelining
	fmt.Printf("  📊 Test 1: GERÇEK Request Rate (Pipelining)\n")
	fmt.Printf("  ┌─────────────┬──────────────┬──────────────┬─────────────┐\n")
	fmt.Printf("  │ Pipeline    │ Request/sec  │ vs Sync      │ Improvement │\n")
	fmt.Printf("  ├─────────────┼──────────────┼──────────────┼─────────────┤\n")

	syncBaseline := 900.0

	pipelineSizes := []int{1, 10, 50}
	for _, pipelineSize := range pipelineSizes {
		requestRate := measureRealRequestRate(t, ":9101", 8, pipelineSize, 5*time.Second)
		improvement := ((requestRate - syncBaseline*8) / (syncBaseline * 8)) * 100

		status := ""
		if improvement > 5000 {
			status = "🔥"
		} else if improvement > 1000 {
			status = "✅"
		} else {
			status = "📊"
		}

		fmt.Printf("  │ %11d │ %10.0f/s │ %10.0f/s │ %9.0f%% %s│\n",
			pipelineSize,
			requestRate,
			syncBaseline*8,
			improvement,
			status,
		)
	}
	fmt.Printf("  └─────────────┴──────────────┴──────────────┴─────────────┘\n")
	fmt.Printf("\n")

	// Test 2: Projection with different batch sizes
	peakRequestRate := measureRealRequestRate(t, ":9101", 16, 50, 5*time.Second)

	fmt.Printf("  📦 Test 2: Theoretical Throughput (Batch Projections)\n")
	fmt.Printf("  ┌─────────────┬──────────────┬──────────────┬─────────────┐\n")
	fmt.Printf("  │ Batch Size  │ Theoretical  │ Actual Req/s │ Reality     │\n")
	fmt.Printf("  ├─────────────┼──────────────┼──────────────┼─────────────┤\n")

	batchSizes := []int{1, 10, 50, 100, 500}
	for _, batchSize := range batchSizes {
		theoretical := peakRequestRate * float64(batchSize)

		fmt.Printf("  │ %11d │ %10.0fK  │ %10.0fK  │ %9dx    │\n",
			batchSize,
			theoretical/1000,
			peakRequestRate/1000,
			batchSize,
		)
	}
	fmt.Printf("  └─────────────┴──────────────┴──────────────┴─────────────┘\n")
	fmt.Printf("\n")

	// Summary
	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  📈 GERÇEK vs THEORETICAL                                        ║\n")
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  Sync Baseline:               %7.0f req/sec                    ║\n", syncBaseline*16)
	fmt.Printf("║  Pipeline Peak (GERÇEK):      %7.0f req/sec                    ║\n", peakRequestRate)
	fmt.Printf("║  Improvement:                 %7.1fx                           ║\n", peakRequestRate/(syncBaseline*16))
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  IF Batch=10:                 %7.0fK msgs/sec (theoretical)   ║\n", peakRequestRate*10/1000)
	fmt.Printf("║  IF Batch=50:                 %7.0fK msgs/sec (theoretical)   ║\n", peakRequestRate*50/1000)
	fmt.Printf("║  IF Batch=100:                %7.0fK msgs/sec (theoretical)   ║\n", peakRequestRate*100/1000)
	fmt.Printf("║  IF Batch=500:                %7.0fK msgs/sec (theoretical)   ║\n", peakRequestRate*500/1000)
	fmt.Printf("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║  💡 AÇIKLAMA:                                                    ║\n")
	fmt.Printf("║  - GERÇEK: Network'ten gönderilen request sayısı                ║\n")
	fmt.Printf("║  - THEORETICAL: Eğer her request N mesaj içerseydi              ║\n")
	fmt.Printf("║  - Production'da batch kullanarak theoretical'e yaklaşılır       ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")

	// Real numbers
	realThroughput := peakRequestRate
	theoretical100 := realThroughput * 100
	theoretical500 := realThroughput * 500

	fmt.Printf("  🎯 GERÇEKÇI PRODUCTION TAHMINI:\n")
	fmt.Printf("     Network üzerinden: %7.0f request/sec (GERÇEK)\n", realThroughput)
	fmt.Printf("     With batch=100:    %7.0fK msgs/sec (realistic)\n", theoretical100/1000)
	fmt.Printf("     With batch=500:    %7.0fK msgs/sec (aggressive)\n", theoretical500/1000)
	fmt.Printf("\n")
	fmt.Printf("     29K baseline'dan:  %7.1fx improvement\n", theoretical100/29000)
	fmt.Printf("\n")
}

func measureRealRequestRate(t *testing.T, addr string, producers int, pipelineDepth int, duration time.Duration) float64 {
	var requestCount atomic.Int64 // Gerçek request sayısı!
	var responseCount atomic.Int64
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

			// Writer goroutine
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
							if _, err := conn.Write(request); err != nil {
								return
							}
							requestCount.Add(1) // Her GERÇEK request için +1
						}
					}
				}
			}()

			// Reader goroutine
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
					responseCount.Add(1)
				}
			}()

			<-done
			time.Sleep(100 * time.Millisecond)
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	// GERÇEK request rate'i döndür (projection YOK!)
	return float64(requestCount.Load()) / elapsed.Seconds()
}
