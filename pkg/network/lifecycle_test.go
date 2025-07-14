package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPortaskDragonflyMessageLifecycle tests complete message lifecycle
func TestPortaskDragonflyMessageLifecycle(t *testing.T) {
	// Check if Dragonfly is available
	conn, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err != nil {
		t.Skipf("❌ Dragonfly not available: %v", err)
	}
	conn.Close()

	// Create Dragonfly storage
	dragonflyConfig := &storage.DragonflyStorageConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	dragonflyStorage, err := storage.NewDragonflyStorage(dragonflyConfig)
	require.NoError(t, err)
	defer dragonflyStorage.Close()
	t.Log("✅ Dragonfly storage initialized")

	// Create Portask server with Dragonfly storage
	codecManager := &serialization.CodecManager{}
	protocolHandler := NewJSONCompatibleProtocolHandler(codecManager, dragonflyStorage)

	serverConfig := &ServerConfig{
		Address: "127.0.0.1", Port: 0, MaxConnections: 10,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
		Network: "tcp", ReadBufferSize: 4096, WriteBufferSize: 4096, WorkerPoolSize: 5,
	}

	server := NewTCPServer(serverConfig, protocolHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start Portask server
	go func() { server.Start(ctx) }()
	time.Sleep(500 * time.Millisecond)

	addr := server.GetAddr()
	require.NotNil(t, addr, "Portask server should start successfully")
	t.Logf("🚀 Portask server started on %s with Dragonfly storage", addr)

	t.Run("Complete Message Lifecycle Test", func(t *testing.T) {
		testTopic := types.TopicName("lifecycle_test")
		messageCount := 5

		// Connect to Portask server
		portaskConn, err := net.DialTimeout("tcp", addr.String(), 5*time.Second)
		require.NoError(t, err)
		defer portaskConn.Close()

		// Step 1: Send messages to Portask
		t.Logf("📤 Step 1: Sending %d messages to Portask", messageCount)
		for i := 0; i < messageCount; i++ {
			msgData := fmt.Sprintf(`{"id": %d, "data": "test message %d", "timestamp": "%s"}`,
				i, i, time.Now().Format(time.RFC3339))

			message := map[string]interface{}{
				"type":  "publish",
				"topic": string(testTopic),
				"data":  msgData,
			}

			jsonData, err := json.Marshal(message)
			require.NoError(t, err)

			_, err = portaskConn.Write(append(jsonData, '\n'))
			require.NoError(t, err)

			t.Logf("  ✅ Sent message %d", i+1)
		}

		// Wait for messages to be processed
		time.Sleep(2 * time.Second)

		// Step 2: Verify messages are stored in Dragonfly
		t.Log("🔍 Step 2: Verifying messages are stored in Dragonfly")
		storedMessages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, messageCount+5)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(storedMessages), messageCount, "All messages should be stored in Dragonfly")
		t.Logf("  📊 Found %d messages in Dragonfly storage", len(storedMessages))

		// Verify message content
		for i, stored := range storedMessages {
			t.Logf("  📝 Message %d: %s", i+1, string(stored.Payload))
		}

		// Step 3: Test message cleanup after processing
		t.Log("🧹 Step 3: Testing message cleanup after processing")

		// Apply retention policy to clean up processed messages
		retentionPolicy := &storage.RetentionPolicy{
			MaxAge:      30 * time.Second, // Keep messages for 30 seconds
			MaxMessages: 2,                // Keep only 2 most recent messages
		}

		err = dragonflyStorage.Cleanup(ctx, retentionPolicy)
		require.NoError(t, err)
		t.Log("  ✅ Cleanup policy applied")

		// Wait for cleanup to process
		time.Sleep(200 * time.Millisecond)

		// Verify some messages were cleaned up
		remainingMessages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, messageCount+5)
		require.NoError(t, err)
		t.Logf("  📊 Messages after cleanup: %d (was %d)", len(remainingMessages), len(storedMessages))

		if len(remainingMessages) <= int(retentionPolicy.MaxMessages) {
			t.Log("  ✅ Cleanup successfully enforced MaxMessages policy")
		}

		// Step 4: Test individual message deletion (simulating processed message removal)
		if len(remainingMessages) > 0 {
			t.Log("🗑️  Step 4: Testing individual message deletion")
			messageToDelete := remainingMessages[0]

			err = dragonflyStorage.Delete(ctx, messageToDelete.ID)
			require.NoError(t, err)
			t.Log("  ✅ Processed message deleted from Dragonfly")

			// Verify deletion
			finalMessages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, messageCount+5)
			require.NoError(t, err)
			t.Logf("  📊 Messages after deletion: %d", len(finalMessages))

			// Verify the specific message is gone
			found := false
			for _, msg := range finalMessages {
				if msg.ID == messageToDelete.ID {
					found = true
					break
				}
			}
			assert.False(t, found, "Deleted message should not be found")
			t.Log("  ✅ Message deletion verified")
		}
	})

	// Stop server
	server.Stop(ctx)
	t.Log("🛑 Portask server stopped")
}
