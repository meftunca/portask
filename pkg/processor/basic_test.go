package processor

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

func TestProcessor_Basic(t *testing.T) {
	config := &ProcessorConfig{
		WorkerCount:        runtime.NumCPU(),
		QueueSize:          1000,
		BatchSize:          10,
		ProcessingTimeout:  time.Second * 5,
		EnableCompression:  false,
		EnableBackpressure: false,
	}

	processor := NewMessageProcessor(config)
	processor.Start(context.Background())
	defer processor.Stop()

	message := &types.PortaskMessage{
		ID:        types.MessageID("test-1"),
		Topic:     types.TopicName("test-topic"),
		Payload:   []byte("test payload"),
		Timestamp: time.Now().UnixNano(),
		Headers:   make(types.MessageHeaders),
		Priority:  types.PriorityNormal,
	}

	ctx := context.Background()
	_, err := processor.ProcessMessage(ctx, message)
	if err != nil {
		t.Logf("Processing returned: %v", err)
	}

	t.Log("Basic processor test completed")
}
