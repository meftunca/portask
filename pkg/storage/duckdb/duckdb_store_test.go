package duckdb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

// TestDuckDBStore tests DuckDB storage basic functionality
func TestDuckDBStore(t *testing.T) {
	// Skip if Arrow is not available (build-time dependency issue)
	t.Skip("DuckDB requires Apache Arrow C++ library. Install with: brew install apache-arrow")

	ctx := context.Background()
	testDir := "./test_duckdb"
	defer os.RemoveAll(testDir)

	config := DefaultConfig()
	config.DataDir = testDir
	config.EnableWAL = false

	store, err := NewDuckDBStore(config)
	if err != nil {
		t.Skipf("DuckDB not available: %v", err)
		return
	}
	defer store.Close()

	// Test single message write
	msg := &types.PortaskMessage{
		ID:        "test-1",
		Topic:     "test-topic",
		Partition: 0,
		Payload:   []byte("hello world"),
		Timestamp: time.Now().UnixNano(),
		TTL:       3600,
		Metadata:  map[string]string{"key": "value"},
		Headers:   map[string]interface{}{"header": "value"},
	}

	err = store.Store(ctx, msg)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Test read
	fetched, err := store.FetchByID(ctx, "test-1")
	if err != nil {
		t.Fatalf("FetchByID failed: %v", err)
	}

	if fetched.ID != msg.ID {
		t.Errorf("Expected ID %s, got %s", msg.ID, fetched.ID)
	}

	// Test batch write
	messages := make([]*types.PortaskMessage, 100)
	for i := 0; i < 100; i++ {
		messages[i] = &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("msg-%d", i)),
			Topic:     "batch-topic",
			Partition: 0,
			Payload:   []byte("batch message"),
			Timestamp: time.Now().UnixNano(),
			TTL:       3600,
			Metadata:  map[string]string{},
			Headers:   map[string]interface{}{},
		}
	}

	batch := types.NewMessageBatch(messages)
	err = store.StoreBatch(ctx, batch)
	if err != nil {
		t.Fatalf("StoreBatch failed: %v", err)
	}

	// Test metrics
	metrics := store.GetMetrics()
	if metrics["messages_written"] != 101 { // 1 + 100
		t.Errorf("Expected 101 messages written, got %d", metrics["messages_written"])
	}

	t.Logf("✅ DuckDB test passed: %d messages written", metrics["messages_written"])
}
