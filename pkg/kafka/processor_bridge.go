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
	processor   *processor.MessageProcessor
	storage     MessageStore
	batchWriter *processor.BatchWriter // Batch writer for optimized storage
}

// NewProcessorBridge creates a new processor bridge
func NewProcessorBridge(proc *processor.MessageProcessor, storage MessageStore) *ProcessorBridge {
	// Create storage adapter for batch writing
	storageAdapter := &KafkaStorageAdapter{Storage: storage}

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

// ProduceMessage processes and stores a message through the processor
func (pb *ProcessorBridge) ProduceMessage(ctx context.Context, msg *types.PortaskMessage) (int64, error) {
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

	return processedMsg.Timestamp, nil
}

// Stop stops the batch writer and flushes remaining messages
func (pb *ProcessorBridge) Stop() error {
	return pb.batchWriter.Stop()
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

// KafkaStorageAdapter adapts MessageStore to processor.StorageBackend interface
type KafkaStorageAdapter struct {
	Storage MessageStore // Exported for external use
}

// StoreBatch implements processor.StorageBackend interface
func (ksa *KafkaStorageAdapter) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	// Write each message in the batch to storage
	for _, msg := range batch.Messages {
		_, err := ksa.Storage.ProduceMessage(
			string(msg.Topic),
			msg.Partition,
			[]byte(msg.Key),
			msg.Payload,
		)
		if err != nil {
			return fmt.Errorf("failed to store message %s: %w", msg.ID, err)
		}
	}
	return nil
}
