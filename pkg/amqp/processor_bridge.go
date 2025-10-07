package amqp

import (
	"context"
	"fmt"

	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/types"
)

// ProcessorBridge bridges AMQP handler with Portask processor and storage
// This ensures all messages go through the processor for validation/processing
type ProcessorBridge struct {
	processor *processor.MessageProcessor
	storage   MessageStore
}

// NewProcessorBridge creates a new processor bridge
func NewProcessorBridge(proc *processor.MessageProcessor, storage MessageStore) *ProcessorBridge {
	return &ProcessorBridge{
		processor: proc,
		storage:   storage,
	}
}

// PublishMessage processes and stores a message through the processor
func (pb *ProcessorBridge) PublishMessage(ctx context.Context, msg *types.PortaskMessage) (int64, error) {
	// 1. Process message through processor (validation, compression, etc.)
	processedMsg, err := pb.processor.ProcessMessage(ctx, msg)
	if err != nil {
		return -1, fmt.Errorf("processor failed: %w", err)
	}

	// 2. Store processed message
	// Convert topic to string for storage
	err = pb.storage.StoreMessage(string(processedMsg.Topic), processedMsg.Payload)
	if err != nil {
		return -1, fmt.Errorf("storage failed: %w", err)
	}

	// Return timestamp as offset
	return processedMsg.Timestamp, nil
}

// ConsumeMessages retrieves messages through storage
func (pb *ProcessorBridge) ConsumeMessages(ctx context.Context, req *types.FetchRequest) ([]*types.PortaskMessage, error) {
	// Fetch from storage
	messages, err := pb.storage.GetMessages(string(req.Topic), req.Offset)
	if err != nil {
		return nil, fmt.Errorf("storage fetch failed: %w", err)
	}

	// Convert to Portask messages
	portaskMessages := make([]*types.PortaskMessage, 0, len(messages))
	for i, msgBytes := range messages {
		portaskMessages = append(portaskMessages, &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("%d", req.Offset+int64(i))),
			Topic:     req.Topic,
			Partition: 0, // AMQP doesn't have partitions
			Payload:   msgBytes,
			Timestamp: req.Offset + int64(i),
		})
	}

	return portaskMessages, nil
}

