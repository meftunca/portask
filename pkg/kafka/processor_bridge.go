package kafka

import (
	"context"
	"fmt"

	"github.com/meftunca/portask/pkg/processor"
	"github.com/meftunca/portask/pkg/types"
)

// ProcessorBridge bridges Kafka handler with Portask processor and storage
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

// ProduceMessage processes and stores a message through the processor
func (pb *ProcessorBridge) ProduceMessage(ctx context.Context, msg *types.PortaskMessage) (int64, error) {
	// 1. Process message through processor (validation, compression, etc.)
	processedMsg, err := pb.processor.ProcessMessage(ctx, msg)
	if err != nil {
		return -1, fmt.Errorf("processor failed: %w", err)
	}

	// 2. Store processed message
	offset, err := pb.storage.ProduceMessage(
		string(processedMsg.Topic),
		processedMsg.Partition,
		[]byte(processedMsg.Key),
		processedMsg.Payload,
	)
	if err != nil {
		return -1, fmt.Errorf("storage failed: %w", err)
	}

	return offset, nil
}

// FetchMessages retrieves messages through storage
func (pb *ProcessorBridge) FetchMessages(ctx context.Context, req *types.FetchRequest) ([]*types.PortaskMessage, error) {
	// Fetch from storage
	kafkaMessages, err := pb.storage.ConsumeMessages(string(req.Topic), req.Partition, req.Offset, int32(req.Limit*1024))
	if err != nil {
		return nil, fmt.Errorf("storage fetch failed: %w", err)
	}

	// Convert Kafka messages to Portask messages
	portaskMessages := make([]*types.PortaskMessage, 0, len(kafkaMessages))
	for _, kmsg := range kafkaMessages {
		portaskMessages = append(portaskMessages, &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("%d", kmsg.Offset)),
			Topic:     req.Topic,
			Partition: req.Partition,
			Key:       string(kmsg.Key),
			Payload:   kmsg.Value,
			Timestamp: kmsg.Offset, // Use offset as timestamp for now
		})
	}

	return portaskMessages, nil
}

