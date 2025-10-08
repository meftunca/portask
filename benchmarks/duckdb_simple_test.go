package benchmarks

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/storage/duckdb"
	"github.com/meftunca/portask/pkg/types"
)

// TestDuckDBSimple tests DuckDB raw performance
func TestDuckDBSimple(t *testing.T) {
	ctx := context.Background()
	messageCount := 50000
	payload := make([]byte, 1024)

	// Clean up
	testDir := "./test_duckdb_simple"
	defer os.RemoveAll(testDir)

	config := duckdb.DefaultConfig()
	config.DataDir = testDir
	config.EnableWAL = false // FASTEST!
	config.EnableCompression = true
	config.MemoryLimit = "2GB"

	duckStore, err := duckdb.NewDuckDBStore(config)
	if err != nil {
		t.Fatalf("Failed to create DuckDB: %v", err)
		return
	}
	defer duckStore.Close()

	t.Logf("")
	t.Logf("🦆 DUCKDB DIRECT BATCH TEST")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Messages: %d (1KB each)", messageCount)
	t.Logf("Config: WAL disabled, compression enabled, 2GB memory")
	t.Logf("")

	// Create batch
	messages := make([]*types.PortaskMessage, messageCount)
	for i := 0; i < messageCount; i++ {
		messages[i] = &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("msg-%d", i)),
			Topic:     types.TopicName(fmt.Sprintf("topic-%d", i%50)),
			Partition: 0,
			Key:       fmt.Sprintf("key-%d", i),
			Payload:   payload,
			Timestamp: time.Now().UnixNano(),
			TTL:       int64(time.Hour),
			Priority:  types.PriorityNormal,
			Status:    types.StatusPending,
			Metadata:  map[string]string{"source": "test"},
			Headers:   map[string]interface{}{"test": "value"},
		}
	}

	batch := types.NewMessageBatch(messages)

	t.Logf("Writing batch...")
	start := time.Now()
	err = duckStore.StoreBatch(ctx, batch)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Batch insert failed: %v", err)
		return
	}

	throughput := float64(messageCount) / duration.Seconds()

	t.Logf("")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("📊 WRITE PERFORMANCE")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("")
	t.Logf("Duration:      %v", duration)
	t.Logf("Throughput:    %.0f msgs/sec", throughput)
	t.Logf("Avg per msg:   %v", duration/time.Duration(messageCount))
	t.Logf("")

	duckMetrics := duckStore.GetMetrics()
	t.Logf("Messages written: %d", duckMetrics["messages_written"])
	t.Logf("Bytes written:    %.2f MB", float64(duckMetrics["bytes_written"])/(1024*1024))
	t.Logf("")

	// Test read performance
	t.Logf("Testing read performance...")
	readStart := time.Now()
	readCount := 1000

	for i := 0; i < readCount; i++ {
		msgID := types.MessageID(fmt.Sprintf("msg-%d", i))
		msg, err := duckStore.FetchByID(ctx, msgID)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
			return
		}
		if msg.ID != msgID {
			t.Fatalf("Wrong message returned")
		}
	}

	readDuration := time.Since(readStart)
	readThroughput := float64(readCount) / readDuration.Seconds()

	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("📖 READ PERFORMANCE")
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("")
	t.Logf("Read count:    %d messages", readCount)
	t.Logf("Duration:      %v", readDuration)
	t.Logf("Throughput:    %.0f msgs/sec", readThroughput)
	t.Logf("Avg per read:  %v", readDuration/time.Duration(readCount))
	t.Logf("")

	// Optimize
	t.Logf("Running ANALYZE optimization...")
	if err := duckStore.Optimize(ctx); err != nil {
		t.Logf("  Warning: %v", err)
	} else {
		t.Logf("  ✅ Database optimized")
	}

	// Stats
	stats, err := duckStore.Stats(ctx)
	if err != nil {
		t.Logf("Warning: stats failed: %v", err)
	} else {
		t.Logf("")
		t.Logf("Status: %s", stats.Status)
		t.Logf("Operations: %d", stats.TotalOperations)
	}

	t.Logf("")
	t.Logf("🦆 Test complete!")
	t.Logf("")
}
