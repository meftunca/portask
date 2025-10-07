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
	processor   *processor.MessageProcessor
	storage     MessageStore
	batchWriter *processor.BatchWriter // Batch writer for optimized storage
}

// NewProcessorBridge creates a new processor bridge
func NewProcessorBridge(proc *processor.MessageProcessor, storage MessageStore) *ProcessorBridge {
	// Create storage adapter for batch writing
	storageAdapter := &AMQPStorageAdapter{storage: storage}
	
	// Create batch writer
	batchWriter := processor.NewBatchWriter(storageAdapter, processor.DefaultBatchWriterConfig())
	
	// Start batch writer
	batchWriter.Start(context.Background())
	
	return &ProcessorBridge{
		processor:   proc,
		storage:     storage,
		batchWriter: batchWriter,
	}
}

// PublishMessage processes and stores a message through the processor
func (pb *ProcessorBridge) PublishMessage(ctx context.Context, msg *types.PortaskMessage) (int64, error) {
	// 1. Process message through processor (validation, compression, etc.)
	processedMsg, err := pb.processor.ProcessMessage(ctx, msg)
	if err != nil {
		return -1, fmt.Errorf("processor failed: %w", err)
	}

	// 2. Write to batch writer (will flush every 10ms or 1000 messages)
	err = pb.batchWriter.Write(processedMsg)
	if err != nil {
		return -1, fmt.Errorf("batch write failed: %w", err)
	}

	// Return timestamp as offset
	return processedMsg.Timestamp, nil
}

// Stop stops the batch writer and flushes remaining messages
func (pb *ProcessorBridge) Stop() error {
	return pb.batchWriter.Stop()
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

// AMQPStorageAdapter adapts MessageStore to processor.StorageBackend interface
type AMQPStorageAdapter struct {
	storage MessageStore
}

// StoreBatch implements processor.StorageBackend interface
func (asa *AMQPStorageAdapter) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	// Write each message in the batch to storage
	for _, msg := range batch.Messages {
		err := asa.storage.StoreMessage(string(msg.Topic), msg.Payload)
		if err != nil {
			return fmt.Errorf("failed to store message %s: %w", msg.ID, err)
		}
	}
	return nil
}

