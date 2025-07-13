package queue

import (
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

func TestLockFreeQueue_Basic(t *testing.T) {
	queue := NewLockFreeQueue("test", 16, DropOldest)

	// Test empty queue
	if !queue.IsEmpty() {
		t.Error("New queue should be empty")
	}

	// Test single enqueue/dequeue
	message := &types.PortaskMessage{
		ID:        types.MessageID("test-1"),
		Topic:     types.TopicName("test"),
		Timestamp: time.Now().UnixNano(),
		Payload:   []byte("test"),
	}

	if !queue.Enqueue(message) {
		t.Error("Failed to enqueue message")
	}

	if queue.IsEmpty() {
		t.Error("Queue should not be empty after enqueue")
	}

	dequeued, ok := queue.Dequeue()
	if !ok {
		t.Error("Failed to dequeue message")
	}

	if dequeued.ID != message.ID {
		t.Errorf("Dequeued message ID mismatch: expected %s, got %s", message.ID, dequeued.ID)
	}

	if !queue.IsEmpty() {
		t.Error("Queue should be empty after dequeue")
	}

	t.Log("LockFreeQueue basic test completed successfully")
}
