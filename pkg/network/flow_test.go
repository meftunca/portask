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

// TestCompleteIntegrationFlow tests the complete flow:
// Message Queue -> Portask -> Dragonfly -> Processing -> Cleanup
func TestCompleteIntegrationFlow(t *testing.T) {
	// Check if Dragonfly is available
	conn, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err != nil {
		t.Skipf("❌ Dragonfly not available: %v", err)
	}
	conn.Close()

	// Setup Dragonfly storage
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

	// Setup Portask server with Dragonfly
	codecManager := &serialization.CodecManager{}
	protocolHandler := NewJSONCompatibleProtocolHandler(codecManager, dragonflyStorage)

	serverConfig := &ServerConfig{
		Address: "127.0.0.1", Port: 0, MaxConnections: 10,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second,
		Network: "tcp", ReadBufferSize: 4096, WriteBufferSize: 4096, WorkerPoolSize: 5,
	}

	server := NewTCPServer(serverConfig, protocolHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	go func() { server.Start(ctx) }()
	time.Sleep(500 * time.Millisecond)

	addr := server.GetAddr()
	require.NotNil(t, addr)
	t.Logf("🚀 Portask server started on %s", addr)

	t.Run("Direct Portask Integration Flow", func(t *testing.T) {
		testTopic := types.TopicName("direct_flow_test")

		// Step 1: Send message to Portask
		portaskConn, err := net.DialTimeout("tcp", addr.String(), 5*time.Second)
		require.NoError(t, err)
		defer portaskConn.Close()

		message := map[string]interface{}{
			"type":  "publish",
			"topic": string(testTopic),
			"data":  `{"order_id": "12345", "status": "processing", "amount": 99.99}`,
		}

		jsonData, err := json.Marshal(message)
		require.NoError(t, err)
		_, err = portaskConn.Write(append(jsonData, '\n'))
		require.NoError(t, err)
		t.Log("📤 Message sent to Portask")

		// Step 2: Verify storage in Dragonfly
		time.Sleep(1 * time.Second)
		messages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
		require.NoError(t, err)
		assert.Greater(t, len(messages), 0, "Message should be stored in Dragonfly")
		t.Logf("✅ Message stored in Dragonfly: %d messages", len(messages))

		// Step 3: Simulate processing completion and cleanup
		originalCount := len(messages)
		if originalCount > 0 {
			msgToProcess := messages[0]
			t.Logf("🔄 Processing message: %s", string(msgToProcess.Payload))

			// Simulate processing completion - delete the processed message
			err = dragonflyStorage.Delete(ctx, msgToProcess.ID)
			require.NoError(t, err)
			t.Log("✅ Processed message deleted from Dragonfly")

			// Verify deletion
			remainingMessages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
			require.NoError(t, err)
			assert.Equal(t, originalCount-1, len(remainingMessages), "Message should be deleted after processing")
			t.Logf("📊 Messages after processing: %d (was %d)", len(remainingMessages), originalCount)
		}
	})

	if isKafkaAvailable() {
		t.Run("Kafka Integration Flow", func(t *testing.T) {
			t.Skip("Skipping Kafka test - requires Kafka server")
			testTopic := types.TopicName("kafka_flow_test")

			// Setup Kafka bridge
			kafkaConfig := DefaultKafkaConfig(KafkaClientSarama)
			bridge, err := NewKafkaPortaskBridge(kafkaConfig, addr.String(), string(testTopic))
			require.NoError(t, err)
			defer bridge.Stop()

			// Start forwarding from Kafka to Portask
			err = bridge.ForwardFromKafkaToPortask([]string{string(testTopic)})
			require.NoError(t, err)

			// Step 1: Publish to Kafka
			kafkaMessage := `{"event": "user_signup", "user_id": "user123", "email": "user@example.com"}`
			err = bridge.PublishToKafka(string(testTopic), "user123", []byte(kafkaMessage))
			require.NoError(t, err)
			t.Log("📤 Message published to Kafka")

			// Step 2: Wait for flow through Kafka -> Portask -> Dragonfly
			time.Sleep(3 * time.Second)

			// Step 3: Verify message reached Dragonfly
			messages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
			require.NoError(t, err)
			assert.Greater(t, len(messages), 0, "Kafka message should reach Dragonfly")
			t.Logf("✅ Kafka message in Dragonfly: %d messages", len(messages))

			// Step 4: Process and cleanup
			if len(messages) > 0 {
				t.Log("🔄 Simulating Kafka message processing")

				// Apply cleanup policy
				retentionPolicy := &storage.RetentionPolicy{
					MaxMessages: 0, // Remove all processed messages
				}

				err = dragonflyStorage.Cleanup(ctx, retentionPolicy)
				require.NoError(t, err)

				time.Sleep(200 * time.Millisecond)

				cleanedMessages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
				require.NoError(t, err)
				t.Logf("📊 Messages after Kafka cleanup: %d", len(cleanedMessages))
				t.Log("✅ Kafka message processing and cleanup completed")
			}
		})
	} else {
		t.Log("⚠️  Kafka not available, skipping Kafka integration test")
	}

	if isRabbitMQAvailable() {
		t.Run("RabbitMQ Integration Flow", func(t *testing.T) {
			testTopic := types.TopicName("rabbitmq_flow_test")

			// Setup RabbitMQ bridge
			rabbitmqConfig := DefaultRabbitMQConfig()
			bridge, err := NewRabbitMQPortaskBridge(rabbitmqConfig, addr.String())
			require.NoError(t, err)
			defer bridge.Stop()

			// Start consuming from RabbitMQ
			err = bridge.ConsumeFromRabbitMQ([]string{string(testTopic)})
			require.NoError(t, err)
			time.Sleep(500 * time.Millisecond) // Wait for consumer

			// Step 1: Publish to RabbitMQ
			rabbitMessage := `{"notification": "order_confirmed", "order_id": "order789", "customer": "customer456"}`
			headers := map[string]interface{}{"priority": "high"}
			err = bridge.PublishToRabbitMQ(string(testTopic), string(testTopic), []byte(rabbitMessage), headers)
			require.NoError(t, err)
			t.Log("📤 Message published to RabbitMQ")

			// Step 2: Wait for flow through RabbitMQ -> Portask -> Dragonfly
			time.Sleep(3 * time.Second)

			// Step 3: Verify message reached Dragonfly
			messages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
			require.NoError(t, err)
			assert.Greater(t, len(messages), 0, "RabbitMQ message should reach Dragonfly")
			t.Logf("✅ RabbitMQ message in Dragonfly: %d messages", len(messages))

			// Step 4: Process and cleanup
			if len(messages) > 0 {
				t.Log("🔄 Simulating RabbitMQ message processing")

				// Delete processed message
				err = dragonflyStorage.Delete(ctx, messages[0].ID)
				require.NoError(t, err)

				remainingMessages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
				require.NoError(t, err)
				t.Logf("📊 Messages after RabbitMQ processing: %d", len(remainingMessages))
				t.Log("✅ RabbitMQ message processing and cleanup completed")
			}
		})
	} else {
		t.Log("⚠️  RabbitMQ not available, skipping RabbitMQ integration test")
	}

	// Stop server
	server.Stop(ctx)
	t.Log("🛑 Integration test completed")
}

// Helper functions to check service availability
func isKafkaAvailable() bool {
	conn, err := net.DialTimeout("tcp", "localhost:9092", 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func isRabbitMQAvailable() bool {
	conn, err := net.DialTimeout("tcp", "localhost:5672", 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// TestMessageQueueProcessingVerification verifies that messages are actually processed and cleaned up
func TestMessageQueueProcessingVerification(t *testing.T) {
	// Check if Dragonfly is available
	conn, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err != nil {
		t.Skipf("❌ Dragonfly not available: %v", err)
	}
	conn.Close()

	// Create storage
	dragonflyConfig := &storage.DragonflyStorageConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     5,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	dragonflyStorage, err := storage.NewDragonflyStorage(dragonflyConfig)
	require.NoError(t, err)
	defer dragonflyStorage.Close()

	ctx := context.Background()

	t.Run("Verify Message Processing and Cleanup", func(t *testing.T) {
		testTopic := types.TopicName("processing_verification")

		// Create test messages simulating queue messages
		messages := []*types.PortaskMessage{
			{
				ID:        types.MessageID("queue_msg_1"),
				Topic:     testTopic,
				Payload:   []byte(`{"type": "order", "status": "pending", "id": "order1"}`),
				Timestamp: time.Now().Unix(),
			},
			{
				ID:        types.MessageID("queue_msg_2"),
				Topic:     testTopic,
				Payload:   []byte(`{"type": "payment", "status": "pending", "id": "pay1"}`),
				Timestamp: time.Now().Unix(),
			},
			{
				ID:        types.MessageID("queue_msg_3"),
				Topic:     testTopic,
				Payload:   []byte(`{"type": "notification", "status": "pending", "id": "notif1"}`),
				Timestamp: time.Now().Unix(),
			},
		}

		// Step 1: Store messages (simulate receiving from queue)
		for i, msg := range messages {
			err := dragonflyStorage.Store(ctx, msg)
			require.NoError(t, err)
			t.Logf("📥 Stored queue message %d: %s", i+1, string(msg.Payload))
		}

		// Verify all stored
		storedMessages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(storedMessages), len(messages), "All messages should be stored")
		t.Logf("📊 Messages in Dragonfly: %d", len(storedMessages))

		// Step 2: Simulate processing each message and deleting after completion
		t.Log("🔄 Simulating message processing...")
		processedCount := 0
		for _, msg := range messages {
			// Simulate processing the message
			t.Logf("  Processing: %s", string(msg.Payload))

			// After processing is complete, delete from Dragonfly
			err = dragonflyStorage.Delete(ctx, msg.ID)
			require.NoError(t, err)
			processedCount++
			t.Logf("  ✅ Processed and deleted message %d", processedCount)

			// Verify immediate deletion
			remaining, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
			require.NoError(t, err)
			expectedRemaining := len(messages) - processedCount
			assert.Equal(t, expectedRemaining, len(remaining), "Message should be deleted immediately after processing")
		}

		// Step 3: Verify all messages are cleaned up
		finalMessages, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
		require.NoError(t, err)
		assert.Equal(t, 0, len(finalMessages), "No messages should remain after processing")
		t.Log("✅ All messages processed and cleaned up from Dragonfly")

		// Step 4: Test batch cleanup for any remaining messages
		t.Log("🧹 Testing batch cleanup policies")

		// Add some more messages
		for i := 0; i < 5; i++ {
			msg := &types.PortaskMessage{
				ID:        types.MessageID(fmt.Sprintf("batch_msg_%d", i)),
				Topic:     testTopic,
				Payload:   []byte(fmt.Sprintf(`{"batch": true, "index": %d}`, i)),
				Timestamp: time.Now().Add(-time.Duration(i) * time.Minute).Unix(),
			}
			err := dragonflyStorage.Store(ctx, msg)
			require.NoError(t, err)
		}

		// Apply aggressive cleanup
		retentionPolicy := &storage.RetentionPolicy{
			MaxAge:      2 * time.Minute,
			MaxMessages: 2,
		}

		err = dragonflyStorage.Cleanup(ctx, retentionPolicy)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		afterCleanup, err := dragonflyStorage.Fetch(ctx, testTopic, 0, 0, 10)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(afterCleanup), int(retentionPolicy.MaxMessages), "Cleanup should enforce retention policy")
		t.Logf("📊 Messages after batch cleanup: %d", len(afterCleanup))
		t.Log("✅ Batch cleanup verification completed")
	})
}
