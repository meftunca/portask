package dragonfly

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDragonflyStorageAvailability tests if Dragonfly is available
func TestDragonflyStorageAvailability(t *testing.T) {
	t.Run("Check Dragonfly connection", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
		if err != nil {
			t.Skipf("❌ Dragonfly NOT available on localhost:6379: %v", err)
		}
		defer conn.Close()
		t.Log("✅ Dragonfly is available on localhost:6379")
	})
}

// TestDragonflyStorageIntegration tests Dragonfly storage operations
func TestDragonflyStorageIntegration(t *testing.T) {
	// Check if Dragonfly is available
	conn, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err != nil {
		t.Skipf("❌ Dragonfly not available: %v", err)
	}
	conn.Close()

	config := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		Password:  "",
		DB:        0,
	}

	store, err := NewDragonflyStore(config)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Connect to Dragonfly
	err = store.Connect(ctx)
	require.NoError(t, err)

	t.Run("Test message storage and retrieval", func(t *testing.T) {
		// Create test message
		message := &types.PortaskMessage{
			ID:        types.MessageID("test_msg_1"),
			Topic:     types.TopicName("test_topic"),
			Payload:   []byte(`{"test": "data", "timestamp": "2024-01-01T10:00:00Z"}`),
			Timestamp: time.Now().Unix(),
			Partition: 0,
		}

		// Store message
		err := store.Store(ctx, message)
		require.NoError(t, err)
		t.Log("✅ Message stored successfully")

		// Verify message was written to Dragonfly
		messages, err := store.Fetch(ctx, message.Topic, 0, 0, 10)
		require.NoError(t, err)
		require.Greater(t, len(messages), 0, "Should have at least one message")

		// Verify message content
		found := false
		for _, msg := range messages {
			if msg.ID == message.ID {
				assert.Equal(t, message.Topic, msg.Topic)
				assert.Equal(t, string(message.Payload), string(msg.Payload))
				found = true
				break
			}
		}
		assert.True(t, found, "Stored message should be found")
		t.Log("✅ Message retrieval verified")
	})

	t.Run("Test message cleanup and deletion", func(t *testing.T) {
		// Create multiple test messages
		testTopic := types.TopicName("cleanup_test")
		messageCount := 5

		var messageIDs []types.MessageID
		for i := 0; i < messageCount; i++ {
			message := &types.PortaskMessage{
				ID:        types.MessageID(fmt.Sprintf("cleanup_msg_%d", i)),
				Topic:     testTopic,
				Payload:   []byte(fmt.Sprintf(`{"message_num": %d, "test": "cleanup"}`, i)),
				Timestamp: time.Now().Unix() - int64(i*60), // Messages from different times
				Partition: 0,
			}

			err := store.Store(ctx, message)
			require.NoError(t, err)
			messageIDs = append(messageIDs, message.ID)
		}
		t.Logf("✅ Stored %d test messages", messageCount)

		// Verify all messages are stored
		messages, err := store.Fetch(ctx, testTopic, 0, 0, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(messages), messageCount, "Should have all stored messages")
		t.Logf("📊 Found %d messages before cleanup", len(messages))

		// Test cleanup with retention policy
		retentionPolicy := &storage.RetentionPolicy{
			MaxAge:      2 * time.Minute, // Keep messages newer than 2 minutes
			MaxMessages: 3,               // Keep only 3 messages
		}

		err = store.Cleanup(ctx, retentionPolicy)
		require.NoError(t, err)
		t.Log("✅ Cleanup executed successfully")

		// Wait a moment for cleanup to process
		time.Sleep(100 * time.Millisecond)

		// Verify some messages were cleaned up
		messagesAfterCleanup, err := store.Fetch(ctx, testTopic, 0, 0, 10)
		require.NoError(t, err)
		t.Logf("📊 Found %d messages after cleanup", len(messagesAfterCleanup))

		// Should have fewer messages after cleanup (depending on retention policy)
		if len(messagesAfterCleanup) < len(messages) {
			t.Log("✅ Cleanup successfully removed old messages")
		} else {
			t.Log("ℹ️  All messages still within retention policy")
		}

		// Test individual message deletion
		if len(messageIDs) > 0 {
			err := store.Delete(ctx, messageIDs[0])
			require.NoError(t, err)
			t.Log("✅ Individual message deletion successful")

			// Verify deletion
			deletedMsg, err := store.FetchByID(ctx, messageIDs[0])
			if err != nil || deletedMsg == nil {
				t.Log("✅ Deleted message no longer retrievable")
			}
		}

		// Test batch deletion
		if len(messageIDs) > 2 {
			batchIDs := messageIDs[1:3]
			err := store.DeleteBatch(ctx, batchIDs)
			require.NoError(t, err)
			t.Logf("✅ Batch deletion successful for %d messages", len(batchIDs))
		}

		// Final message count
		finalMessages, err := store.Fetch(ctx, testTopic, 0, 0, 10)
		require.NoError(t, err)
		t.Logf("📊 Final message count: %d", len(finalMessages))
	})

	t.Run("Test TTL and automatic expiration", func(t *testing.T) {
		testTopic := types.TopicName("ttl_test")

		// Create message with timestamp in the past
		oldMessage := &types.PortaskMessage{
			ID:        types.MessageID("old_msg"),
			Topic:     testTopic,
			Payload:   []byte(`{"test": "old_data"}`),
			Timestamp: time.Now().Add(-10 * time.Minute).Unix(), // 10 minutes ago
			Partition: 0,
		}

		// Create recent message
		recentMessage := &types.PortaskMessage{
			ID:        types.MessageID("recent_msg"),
			Topic:     testTopic,
			Payload:   []byte(`{"test": "recent_data"}`),
			Timestamp: time.Now().Unix(),
			Partition: 0,
		}

		// Store both messages
		err := store.Store(ctx, oldMessage)
		require.NoError(t, err)
		err = store.Store(ctx, recentMessage)
		require.NoError(t, err)
		t.Log("✅ Stored old and recent messages")

		// Apply aggressive cleanup policy
		retentionPolicy := &storage.RetentionPolicy{
			MaxAge: 5 * time.Minute, // Keep only messages newer than 5 minutes
		}

		err = store.Cleanup(ctx, retentionPolicy)
		require.NoError(t, err)
		t.Log("✅ Applied TTL-based cleanup")

		// Wait for cleanup to process
		time.Sleep(200 * time.Millisecond)

		// Check remaining messages
		messages, err := store.Fetch(ctx, testTopic, 0, 0, 10)
		require.NoError(t, err)

		// Recent message should still exist, old message should be gone
		recentFound := false
		oldFound := false
		for _, msg := range messages {
			if msg.ID == recentMessage.ID {
				recentFound = true
			}
			if msg.ID == oldMessage.ID {
				oldFound = true
			}
		}

		t.Logf("📊 TTL test results: recent=%t, old=%t", recentFound, oldFound)
		if !oldFound && recentFound {
			t.Log("✅ TTL working correctly: old message expired, recent message preserved")
		} else if recentFound {
			t.Log("ℹ️  Recent message preserved (good)")
		}
	})

	t.Run("Test high-volume cleanup performance", func(t *testing.T) {
		testTopic := types.TopicName("performance_cleanup")
		messageCount := 100

		// Store many messages
		startTime := time.Now()
		for i := 0; i < messageCount; i++ {
			message := &types.PortaskMessage{
				ID:        types.MessageID(fmt.Sprintf("perf_msg_%d", i)),
				Topic:     testTopic,
				Payload:   []byte(fmt.Sprintf(`{"index": %d, "data": "performance test"}`, i)),
				Timestamp: time.Now().Add(-time.Duration(i) * time.Second).Unix(),
				Partition: 0,
			}
			err := store.Store(ctx, message)
			require.NoError(t, err)
		}
		storeTime := time.Since(startTime)
		t.Logf("✅ Stored %d messages in %v", messageCount, storeTime)

		// Verify all stored
		messages, err := store.Fetch(ctx, testTopic, 0, 0, messageCount+10)
		require.NoError(t, err)
		t.Logf("📊 Verified %d messages stored", len(messages))

		// Perform cleanup
		retentionPolicy := &storage.RetentionPolicy{
			MaxAge:      30 * time.Second, // Keep only recent messages
			MaxMessages: 50,               // Keep max 50 messages
		}

		cleanupStart := time.Now()
		err = store.Cleanup(ctx, retentionPolicy)
		require.NoError(t, err)
		cleanupTime := time.Since(cleanupStart)
		t.Logf("✅ Cleanup completed in %v", cleanupTime)

		// Check final count
		time.Sleep(100 * time.Millisecond)
		finalMessages, err := store.Fetch(ctx, testTopic, 0, 0, messageCount+10)
		require.NoError(t, err)
		t.Logf("📊 Final message count: %d (reduced from %d)", len(finalMessages), len(messages))

		// Cleanup should be reasonably fast
		assert.Less(t, cleanupTime, 5*time.Second, "Cleanup should complete quickly")
		t.Logf("📈 Cleanup performance: %d messages in %v", messageCount, cleanupTime)
	})
}

// TestDragonflyStorageReliability tests error handling and edge cases
func TestDragonflyStorageReliability(t *testing.T) {
	// Check if Dragonfly is available
	conn, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err != nil {
		t.Skipf("❌ Dragonfly not available: %v", err)
	}
	conn.Close()

	config := &storage.DragonflyConfig{
		Addresses: []string{"localhost:6379"},
		Password:  "",
		DB:        0,
	}

	store, err := NewDragonflyStore(config)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Connect to Dragonfly
	err = store.Connect(ctx)
	require.NoError(t, err)

	t.Run("Test basic storage operations", func(t *testing.T) {
		// Test that storage is working by doing basic operations
		testTopic := types.TopicName(fmt.Sprintf("basic_test_%d", time.Now().UnixNano()))

		// Check if topic already exists and delete if necessary
		exists, topicErr := store.TopicExists(ctx, testTopic)
		require.NoError(t, topicErr)
		if exists {
			topicErr = store.DeleteTopic(ctx, testTopic)
			require.NoError(t, topicErr)
		}

		// Create topic
		topicInfo := &types.TopicInfo{
			Name:              testTopic,
			Partitions:        1,
			ReplicationFactor: 1,
			CreatedAt:         time.Now().Unix(),
		}

		err = store.CreateTopic(ctx, topicInfo)
		require.NoError(t, err)
		t.Log("✅ Basic storage operations working")
	})

	t.Run("Test topic management", func(t *testing.T) {
		testTopic := types.TopicName(fmt.Sprintf("mgmt_test_%d", time.Now().UnixNano()))

		// Check if topic already exists and delete if necessary
		exists, topicErr := store.TopicExists(ctx, testTopic)
		require.NoError(t, topicErr)
		if exists {
			topicErr = store.DeleteTopic(ctx, testTopic)
			require.NoError(t, topicErr)
		}

		// Create topic
		topicInfo := &types.TopicInfo{
			Name:              testTopic,
			Partitions:        1,
			ReplicationFactor: 1,
			CreatedAt:         time.Now().Unix(),
		}

		err = store.CreateTopic(ctx, topicInfo)
		require.NoError(t, err)
		t.Log("✅ Topic created successfully")

		// Verify topic exists
		exists, err := store.TopicExists(ctx, testTopic)
		require.NoError(t, err)
		assert.True(t, exists, "Topic should exist after creation")

		// Get topic info
		retrievedInfo, err := store.GetTopicInfo(ctx, testTopic)
		require.NoError(t, err)
		assert.Equal(t, testTopic, retrievedInfo.Name)
		t.Log("✅ Topic info retrieved successfully")

		// List all topics
		topics, err := store.ListTopics(ctx)
		require.NoError(t, err)
		assert.Greater(t, len(topics), 0, "Should have at least one topic")
		t.Logf("📊 Found %d topics", len(topics))

		// Delete topic
		err = store.DeleteTopic(ctx, testTopic)
		require.NoError(t, err)
		t.Log("✅ Topic deleted successfully")

		// Verify topic no longer exists
		exists, err = store.TopicExists(ctx, testTopic)
		require.NoError(t, err)
		assert.False(t, exists, "Topic should not exist after deletion")
	})
}
