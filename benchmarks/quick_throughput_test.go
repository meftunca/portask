package benchmarks

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
)

// Quick throughput test - no infinity loops!
func TestQuickThroughput(t *testing.T) {
	store := NewMockThroughputStore()
	
	scenarios := []struct {
		name        string
		duration    time.Duration
		concurrency int
		messageSize int
	}{
		{"Sequential", 1 * time.Second, 1, 128},
		{"10 Concurrent", 1 * time.Second, 10, 128},
		{"100 Concurrent", 1 * time.Second, 100, 128},
		{"1000 Concurrent", 1 * time.Second, 1000, 128},
	}

	fmt.Println("\n╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                                  ║")
	fmt.Println("║           🚀 PORTASK KAFKA - QUICK THROUGHPUT TEST 🚀           ║")
	fmt.Println("║                                                                  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var totalMessages int64
			var wg sync.WaitGroup

			message := make([]byte, scenario.messageSize)
			for i := range message {
				message[i] = byte('A' + (i % 26))
			}

			start := time.Now()
			deadline := start.Add(scenario.duration)

			// Launch workers
			for i := 0; i < scenario.concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					count := int64(0)
					for time.Now().Before(deadline) {
						store.ProduceMessage("test-topic", 0, nil, message)
						count++
					}
					atomic.AddInt64(&totalMessages, count)
				}()
			}

			wg.Wait()
			elapsed := time.Since(start)

			throughput := float64(totalMessages) / elapsed.Seconds()
			mbPerSec := float64(totalMessages*int64(scenario.messageSize)) / elapsed.Seconds() / 1024 / 1024

			fmt.Printf("📊 %s:\n", scenario.name)
			fmt.Printf("   ├─ Messages:    %s\n", formatNumber(totalMessages))
			fmt.Printf("   ├─ Throughput:  %s messages/sec\n", formatNumber(int64(throughput)))
			fmt.Printf("   ├─ Bandwidth:   %.2f MB/sec\n", mbPerSec)
			fmt.Printf("   └─ Latency:     %.2f µs/msg\n\n", 1000000.0/throughput)
		})
	}
}

func TestOffsetCommitThroughput(t *testing.T) {
	offsetManager := kafka.NewOffsetManagerWithMetadata()
	
	scenarios := []struct {
		name        string
		duration    time.Duration
		concurrency int
	}{
		{"Sequential", 1 * time.Second, 1},
		{"10 Concurrent", 1 * time.Second, 10},
		{"100 Concurrent", 1 * time.Second, 100},
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           📝 OFFSET COMMIT THROUGHPUT TEST 📝                    ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var totalCommits int64
			var wg sync.WaitGroup

			start := time.Now()
			deadline := start.Add(scenario.duration)

			for i := 0; i < scenario.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					count := int64(0)
					offset := int64(0)
					for time.Now().Before(deadline) {
						offsetManager.CommitOffset(
							fmt.Sprintf("group-%d", workerID),
							"test-topic",
							0,
							offset,
						)
						offset++
						count++
					}
					atomic.AddInt64(&totalCommits, count)
				}(i)
			}

			wg.Wait()
			elapsed := time.Since(start)

			throughput := float64(totalCommits) / elapsed.Seconds()

			fmt.Printf("📊 %s:\n", scenario.name)
			fmt.Printf("   ├─ Commits:     %s\n", formatNumber(totalCommits))
			fmt.Printf("   ├─ Throughput:  %s commits/sec\n", formatNumber(int64(throughput)))
			fmt.Printf("   └─ Latency:     %.2f µs/commit\n\n", 1000000.0/throughput)
		})
	}
}

func TestGroupHeartbeatThroughput(t *testing.T) {
	groupCoordinator := kafka.NewGroupCoordinator()
	
	// Setup groups
	for i := 0; i < 10; i++ {
		groupID := fmt.Sprintf("test-group-%d", i)
		memberID := fmt.Sprintf("member-%d", i)
		groupCoordinator.JoinGroup(
			groupID, memberID, "consumer", "roundrobin", "range",
			30000*time.Millisecond, 5000*time.Millisecond,
			[]string{"test-topic"}, nil,
		)
	}

	scenarios := []struct {
		name        string
		duration    time.Duration
		concurrency int
	}{
		{"Sequential", 1 * time.Second, 1},
		{"10 Concurrent", 1 * time.Second, 10},
		{"100 Concurrent", 1 * time.Second, 100},
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           💓 GROUP HEARTBEAT THROUGHPUT TEST 💓                  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var totalHeartbeats int64
			var wg sync.WaitGroup

			start := time.Now()
			deadline := start.Add(scenario.duration)

			for i := 0; i < scenario.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					count := int64(0)
					groupID := fmt.Sprintf("test-group-%d", workerID%10)
					memberID := fmt.Sprintf("member-%d", workerID%10)
					
					for time.Now().Before(deadline) {
						groupCoordinator.Heartbeat(groupID, memberID, 0)
						count++
					}
					atomic.AddInt64(&totalHeartbeats, count)
				}(i)
			}

			wg.Wait()
			elapsed := time.Since(start)

			throughput := float64(totalHeartbeats) / elapsed.Seconds()

			fmt.Printf("📊 %s:\n", scenario.name)
			fmt.Printf("   ├─ Heartbeats:  %s\n", formatNumber(totalHeartbeats))
			fmt.Printf("   ├─ Throughput:  %s heartbeats/sec\n", formatNumber(int64(throughput)))
			fmt.Printf("   └─ Latency:     %.2f µs/heartbeat\n\n", 1000000.0/throughput)
		})
	}
}

func formatNumber(n int64) string {
	if n >= 1000000000 {
		return fmt.Sprintf("%.2fB", float64(n)/1000000000)
	}
	if n >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.2fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

