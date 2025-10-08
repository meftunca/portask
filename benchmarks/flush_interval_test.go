package benchmarks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/kafka"
	"github.com/meftunca/portask/pkg/memory"
	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/storage/dragonfly"
)

// TestFlushIntervalImpact tests different flush intervals
func TestFlushIntervalImpact(t *testing.T) {
	ctx := context.Background()
	translator := kafka.NewKafkaTranslator()
	payload := make([]byte, 1024)
	messageCount := 50000

	// Test various flush intervals
	flushIntervals := []time.Duration{
		5 * time.Millisecond,
		10 * time.Millisecond,
		15 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		50 * time.Millisecond,
	}

	results := make(map[time.Duration]struct {
		throughput float64
		batches    int64
		avgBatch   float64
		latency    time.Duration
	})

	dfConfig := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		DB:        0,
		KeyPrefix: "portask-flush-test",
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

	t.Logf("")
	t.Logf("🔬 FlushInterval Impact Analysis")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Testing %d messages with various flush intervals...", messageCount)
	t.Logf("")

	for _, interval := range flushIntervals {
		interval := interval

		t.Run(fmt.Sprintf("Flush_%dms", interval.Milliseconds()), func(t *testing.T) {
			dragonflyStore.GetClient().FlushDB(ctx)
			time.Sleep(100 * time.Millisecond)

			kafkaStore := NewDragonflyKafkaStore(ctx, dragonflyStore)
			storageAdapter := &kafka.KafkaStorageAdapter{Storage: kafkaStore}

			config := &processor.ParallelBatchWriterConfig{
				NumShards:     32,
				FlushInterval: interval,
				BatchSize:     500,
				MaxRetries:    3,
			}

			asyncWriter := processor.NewAsyncBatchWriter(storageAdapter, config)
			asyncWriter.Start(ctx)

			start := time.Now()
			for i := 0; i < messageCount; i++ {
				msg, _ := translator.TranslateProduce(fmt.Sprintf("topic-%d", i%50), 0, nil, payload)
				asyncWriter.Write(msg)
				memory.PutMessage(msg)
			}

			// Wait time proportional to flush interval
			waitTime := interval * 20
			if waitTime < 200*time.Millisecond {
				waitTime = 200 * time.Millisecond
			}
			time.Sleep(waitTime)

			asyncWriter.Stop()

			duration := time.Since(start)
			throughput := float64(messageCount) / duration.Seconds()

			metrics := asyncWriter.GetMetrics()
			batches := metrics.TotalBatchesWritten.Load()
			avgBatch := float64(messageCount) / float64(batches)
			avgLatency := duration / time.Duration(messageCount)

			results[interval] = struct {
				throughput float64
				batches    int64
				avgBatch   float64
				latency    time.Duration
			}{
				throughput: throughput,
				batches:    batches,
				avgBatch:   avgBatch,
				latency:    avgLatency,
			}

			t.Logf("Interval: %3dms | Throughput: %6.0f msgs/sec | Batches: %3d | Avg Batch: %3.0f | Latency: %v",
				interval.Milliseconds(), throughput, batches, avgBatch, avgLatency)
		})

		time.Sleep(200 * time.Millisecond)
	}

	t.Run("Analysis", func(t *testing.T) {
		t.Logf("")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("📊 FlushInterval Impact Analysis")
		t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.Logf("")

		// Find optimal
		var optimalInterval time.Duration
		var maxThroughput float64

		t.Logf("Results Summary:")
		t.Logf("")
		t.Logf("  Interval | Throughput    | Batches | Avg Batch | Latency    | Efficiency")
		t.Logf("  ---------|---------------|---------|-----------|------------|------------")

		for _, interval := range flushIntervals {
			result := results[interval]
			marker := "  "

			if result.throughput > maxThroughput {
				maxThroughput = result.throughput
				optimalInterval = interval
				marker = "🏆"
			}

			efficiency := (result.avgBatch / 500.0) * 100

			t.Logf("  %s %3dms | %6.0f msgs/s | %3d     | %3.0f/500  | %6v | %5.1f%%",
				marker, interval.Milliseconds(), result.throughput, result.batches,
				result.avgBatch, result.latency, efficiency)
		}

		t.Logf("")
		t.Logf("🎯 Optimal: %dms with %.0f msgs/sec", optimalInterval.Milliseconds(), maxThroughput)
		t.Logf("")

		// Analysis
		t.Logf("💡 Analysis:")
		t.Logf("")

		result5 := results[5*time.Millisecond]
		result10 := results[10*time.Millisecond]
		result20 := results[20*time.Millisecond]

		if result5.avgBatch > 0 && result10.avgBatch > 0 {
			t.Logf("  5ms vs 10ms:")
			t.Logf("    Avg Batch: %.0f → %.0f (+%.0f%%)",
				result5.avgBatch, result10.avgBatch,
				((result10.avgBatch-result5.avgBatch)/result5.avgBatch)*100)
			t.Logf("    Throughput: %.0f → %.0f (+%.0f%%)",
				result5.throughput, result10.throughput,
				((result10.throughput-result5.throughput)/result5.throughput)*100)
			t.Logf("")
		}

		if result10.avgBatch > 0 && result20.avgBatch > 0 {
			t.Logf("  10ms vs 20ms:")
			t.Logf("    Avg Batch: %.0f → %.0f (%.0f%%)",
				result10.avgBatch, result20.avgBatch,
				((result20.avgBatch-result10.avgBatch)/result10.avgBatch)*100)
			t.Logf("    Throughput: %.0f → %.0f (%.0f%%)",
				result10.throughput, result20.throughput,
				((result20.throughput-result10.throughput)/result10.throughput)*100)
			t.Logf("    Latency: %v → %v", result10.latency, result20.latency)
			t.Logf("")
		}

		t.Logf("📝 Conclusions:")
		t.Logf("")

		if result5.avgBatch < 300 {
			t.Logf("  ❌ 5ms: TOO AGGRESSIVE")
			t.Logf("     Batches don't fill (avg %.0f/500)", result5.avgBatch)
			t.Logf("     Many small network calls")
		}

		if result10.avgBatch >= 450 {
			t.Logf("  ✅ 10ms: OPTIMAL")
			t.Logf("     Batches nearly full (avg %.0f/500)", result10.avgBatch)
			t.Logf("     Good balance: throughput + latency")
		}

		if result20.throughput < result10.throughput*0.95 {
			t.Logf("  ⚠️  20ms: TOO SLOW")
			t.Logf("     Latency increases unnecessarily")
			t.Logf("     No throughput benefit")
		} else if result20.throughput >= result10.throughput {
			t.Logf("  ✅ 20ms: ALSO GOOD")
			t.Logf("     Similar throughput to 10ms")
			t.Logf("     Slightly higher latency acceptable for some use cases")
		}

		t.Logf("")
		t.Logf("🎯 Recommendation:")

		if optimalInterval <= 10*time.Millisecond {
			t.Logf("   Use 10ms for best balance")
		} else if optimalInterval <= 20*time.Millisecond {
			t.Logf("   Use 10-20ms depending on latency requirements")
		} else {
			t.Logf("   Use %dms if latency tolerance allows", optimalInterval.Milliseconds())
		}

		t.Logf("")
	})
}
