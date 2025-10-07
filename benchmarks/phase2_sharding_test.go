package benchmarks

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
)

// TestPhase2LockSharding tests lock sharding optimizations
func TestPhase2LockSharding(t *testing.T) {
	fmt.Printf("\n")
	fmt.Printf("╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("║  🔒 PHASE 2: LOCK SHARDING OPTIMIZATION                          ║\n")
	fmt.Printf("║                                                                  ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")

	t.Run("OffsetManagerSharding", func(t *testing.T) {
		testOffsetManagerSharding(t)
	})

	t.Run("GroupCoordinatorSharding", func(t *testing.T) {
		testGroupCoordinatorSharding(t)
	})
}

func testOffsetManagerSharding(t *testing.T) {
	fmt.Printf("  🔍 Testing Offset Manager Sharding...\n\n")

	// Test old (single lock) vs new (sharded locks)
	oldManager := kafka.NewOffsetManagerWithMetadata()
	newManager := kafka.NewShardedOffsetManager()

	concurrencyLevels := []int{1, 4, 16, 64}

	fmt.Printf("  Offset Commit Performance:\n")
	fmt.Printf("  ┌────────────┬─────────────────┬─────────────────┬─────────────┐\n")
	fmt.Printf("  │ Goroutines │ Single Lock     │ Sharded (64)    │ Improvement │\n")
	fmt.Printf("  ├────────────┼─────────────────┼─────────────────┼─────────────┤\n")

	for _, concurrency := range concurrencyLevels {
		oldThroughput := benchmarkOffsetManager(t, oldManager, concurrency)
		newThroughput := benchmarkShardedOffsetManager(t, newManager, concurrency)
		
		improvement := ((newThroughput - oldThroughput) / oldThroughput) * 100

		fmt.Printf("  │ %10d │ %9.0f ops/s │ %9.0f ops/s │ %9.1f%% │\n",
			concurrency,
			oldThroughput,
			newThroughput,
			improvement,
		)
	}

	fmt.Printf("  └────────────┴─────────────────┴─────────────────┴─────────────┘\n\n")

	// Check lock contention reduction
	stats := newManager.GetStats()
	fmt.Printf("  📊 Shard Distribution:\n")
	fmt.Printf("     Total Groups:  %d\n", stats.TotalGroups)
	fmt.Printf("     Total Offsets: %d\n", stats.TotalOffsets)
	fmt.Printf("     Shards:        %d\n", len(stats.ShardStats))
	fmt.Printf("\n")
}

func testGroupCoordinatorSharding(t *testing.T) {
	fmt.Printf("  🔍 Testing Group Coordinator Sharding...\n\n")

	oldCoordinator := kafka.NewGroupCoordinator()
	newCoordinator := kafka.NewShardedGroupCoordinator()

	concurrencyLevels := []int{1, 4, 16, 64}

	fmt.Printf("  Heartbeat Performance:\n")
	fmt.Printf("  ┌────────────┬─────────────────┬─────────────────┬─────────────┐\n")
	fmt.Printf("  │ Goroutines │ Single Lock     │ Sharded (64)    │ Improvement │\n")
	fmt.Printf("  ├────────────┼─────────────────┼─────────────────┼─────────────┤\n")

	for _, concurrency := range concurrencyLevels {
		oldThroughput := benchmarkGroupCoordinator(t, oldCoordinator, concurrency)
		newThroughput := benchmarkShardedGroupCoordinator(t, newCoordinator, concurrency)
		
		improvement := ((newThroughput - oldThroughput) / oldThroughput) * 100

		fmt.Printf("  │ %10d │ %9.0f ops/s │ %9.0f ops/s │ %9.1f%% │\n",
			concurrency,
			oldThroughput,
			newThroughput,
			improvement,
		)
	}

	fmt.Printf("  └────────────┴─────────────────┴─────────────────┴─────────────┘\n\n")

	stats := newCoordinator.GetStats()
	fmt.Printf("  📊 Shard Distribution:\n")
	fmt.Printf("     Total Groups:  %d\n", stats.TotalGroups)
	fmt.Printf("     Total Members: %d\n", stats.TotalMembers)
	fmt.Printf("     Shards:        %d\n", len(stats.ShardStats))
	fmt.Printf("\n")
}

func benchmarkOffsetManager(t *testing.T, manager *kafka.OffsetManagerWithMetadata, concurrency int) float64 {
	duration := 1 * time.Second
	var count atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			groupID := fmt.Sprintf("test-group-%d", id%10) // 10 different groups
			topic := "test-topic"
			
			for time.Since(start) < duration {
				partition := int32(id % 4)
				offset := int64(count.Load())
				
				manager.CommitOffset(groupID, topic, partition, offset)
				count.Add(1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	return float64(count.Load()) / elapsed.Seconds()
}

func benchmarkShardedOffsetManager(t *testing.T, manager *kafka.ShardedOffsetManager, concurrency int) float64 {
	duration := 1 * time.Second
	var count atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			groupID := fmt.Sprintf("test-group-%d", id%10)
			topic := "test-topic"
			
			for time.Since(start) < duration {
				partition := int32(id % 4)
				offset := int64(count.Load())
				
				manager.CommitOffset(groupID, topic, partition, offset)
				count.Add(1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	return float64(count.Load()) / elapsed.Seconds()
}

func benchmarkGroupCoordinator(t *testing.T, coordinator *kafka.GroupCoordinator, concurrency int) float64 {
	duration := 1 * time.Second
	var count atomic.Int64
	var wg sync.WaitGroup

	// Setup: Join groups first
	for i := 0; i < concurrency; i++ {
		groupID := fmt.Sprintf("test-group-%d", i%10)
		memberID := fmt.Sprintf("member-%d", i)
		
		coordinator.JoinGroup(
			groupID, memberID, "test-client", "localhost", "consumer",
			30*time.Second,
			60*time.Second,
			[]string{"test-topic"},
			nil,
		)
	}

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			groupID := fmt.Sprintf("test-group-%d", id%10)
			memberID := fmt.Sprintf("member-%d", id)
			generationID := int32(1)
			
			for time.Since(start) < duration {
				coordinator.Heartbeat(groupID, memberID, generationID)
				count.Add(1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	return float64(count.Load()) / elapsed.Seconds()
}

func benchmarkShardedGroupCoordinator(t *testing.T, coordinator *kafka.ShardedGroupCoordinator, concurrency int) float64 {
	duration := 1 * time.Second
	var count atomic.Int64
	var wg sync.WaitGroup

	// Setup: Join groups first
	for i := 0; i < concurrency; i++ {
		groupID := fmt.Sprintf("test-group-%d", i%10)
		memberID := fmt.Sprintf("member-%d", i)
		
		coordinator.JoinGroup(
			groupID, memberID, "test-client", "localhost", "consumer",
			30*time.Second,
			60*time.Second,
			[]string{"test-topic"},
			nil,
		)
	}

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			groupID := fmt.Sprintf("test-group-%d", id%10)
			memberID := fmt.Sprintf("member-%d", id)
			generationID := int32(1)
			
			for time.Since(start) < duration {
				coordinator.Heartbeat(groupID, memberID, generationID)
				count.Add(1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	return float64(count.Load()) / elapsed.Seconds()
}

