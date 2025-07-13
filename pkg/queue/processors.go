package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/meftunca/portask/pkg/compression"
	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/types"
)

// DefaultMessageProcessor implements a basic message processor
type DefaultMessageProcessor struct {
	codec       serialization.Codec
	compressor  compression.Compressor
	enableDebug bool
}

// NewDefaultMessageProcessor creates a default message processor
func NewDefaultMessageProcessor(codec serialization.Codec, compressor compression.Compressor) *DefaultMessageProcessor {
	return &DefaultMessageProcessor{
		codec:      codec,
		compressor: compressor,
	}
}

// ProcessMessage processes a message
func (p *DefaultMessageProcessor) ProcessMessage(ctx context.Context, message *types.PortaskMessage) error {
	if p.enableDebug {
		log.Printf("Processing message: ID=%s, Topic=%s, Priority=%d, Size=%d bytes",
			message.ID, message.Topic, message.Priority, message.GetSize())
	}

	// Simulate some processing time based on message size
	processingTime := time.Duration(len(message.Payload)/1000) * time.Microsecond
	if processingTime > 10*time.Millisecond {
		processingTime = 10 * time.Millisecond
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(processingTime):
		// Processing complete
	}

	return nil
}

// SetDebug enables/disables debug logging
func (p *DefaultMessageProcessor) SetDebug(enabled bool) {
	p.enableDebug = enabled
}

// BenchmarkMessageProcessor implements a processor for benchmarking
type BenchmarkMessageProcessor struct {
	processedCount uint64
	totalBytes     uint64
	startTime      time.Time
}

// NewBenchmarkMessageProcessor creates a benchmark message processor
func NewBenchmarkMessageProcessor() *BenchmarkMessageProcessor {
	return &BenchmarkMessageProcessor{
		startTime: time.Now(),
	}
}

// ProcessMessage processes a message for benchmarking
func (p *BenchmarkMessageProcessor) ProcessMessage(ctx context.Context, message *types.PortaskMessage) error {
	// Just count messages and bytes - minimal processing
	p.processedCount++
	p.totalBytes += uint64(message.GetSize())
	return nil
}

// GetStats returns benchmark statistics
func (p *BenchmarkMessageProcessor) GetStats() (uint64, uint64, time.Duration) {
	return p.processedCount, p.totalBytes, time.Since(p.startTime)
}

// Reset resets benchmark statistics
func (p *BenchmarkMessageProcessor) Reset() {
	p.processedCount = 0
	p.totalBytes = 0
	p.startTime = time.Now()
}

// EchoMessageProcessor implements a processor that echoes messages
type EchoMessageProcessor struct {
	outputChannel chan<- *types.PortaskMessage
}

// NewEchoMessageProcessor creates an echo message processor
func NewEchoMessageProcessor(outputChannel chan<- *types.PortaskMessage) *EchoMessageProcessor {
	return &EchoMessageProcessor{
		outputChannel: outputChannel,
	}
}

// ProcessMessage processes a message by echoing it to the output channel
func (p *EchoMessageProcessor) ProcessMessage(ctx context.Context, message *types.PortaskMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.outputChannel <- message:
		return nil
	case <-time.After(100 * time.Millisecond):
		// Timeout to avoid deadlock if channel is full
		return fmt.Errorf("output channel send timeout")
	}
}

// BatchMessageProcessor implements batched message processing
type BatchMessageProcessor struct {
	batchProcessor func([]*types.PortaskMessage) error
	batchSize      int
	batchTimeout   time.Duration
	batch          []*types.PortaskMessage
	lastFlush      time.Time
}

// NewBatchMessageProcessor creates a batch message processor
func NewBatchMessageProcessor(
	batchProcessor func([]*types.PortaskMessage) error,
	batchSize int,
	batchTimeout time.Duration,
) *BatchMessageProcessor {
	return &BatchMessageProcessor{
		batchProcessor: batchProcessor,
		batchSize:      batchSize,
		batchTimeout:   batchTimeout,
		batch:          make([]*types.PortaskMessage, 0, batchSize),
		lastFlush:      time.Now(),
	}
}

// ProcessMessage adds a message to the batch
func (p *BatchMessageProcessor) ProcessMessage(ctx context.Context, message *types.PortaskMessage) error {
	p.batch = append(p.batch, message)

	// Check if we should flush the batch
	shouldFlush := len(p.batch) >= p.batchSize ||
		time.Since(p.lastFlush) >= p.batchTimeout

	if shouldFlush {
		return p.flush(ctx)
	}

	return nil
}

// flush processes the current batch
func (p *BatchMessageProcessor) flush(ctx context.Context) error {
	if len(p.batch) == 0 {
		return nil
	}

	err := p.batchProcessor(p.batch)
	p.batch = p.batch[:0] // Reset slice but keep capacity
	p.lastFlush = time.Now()

	return err
}

// Flush manually flushes any pending messages
func (p *BatchMessageProcessor) Flush(ctx context.Context) error {
	return p.flush(ctx)
}

// ChainedMessageProcessor implements a chain of processors
type ChainedMessageProcessor struct {
	processors []MessageProcessor
}

// NewChainedMessageProcessor creates a chained message processor
func NewChainedMessageProcessor(processors ...MessageProcessor) *ChainedMessageProcessor {
	return &ChainedMessageProcessor{
		processors: processors,
	}
}

// ProcessMessage processes a message through the chain
func (p *ChainedMessageProcessor) ProcessMessage(ctx context.Context, message *types.PortaskMessage) error {
	for _, processor := range p.processors {
		if err := processor.ProcessMessage(ctx, message); err != nil {
			return err
		}
	}
	return nil
}

// Add adds a processor to the chain
func (p *ChainedMessageProcessor) Add(processor MessageProcessor) {
	p.processors = append(p.processors, processor)
}

// FilterMessageProcessor implements message filtering
type FilterMessageProcessor struct {
	filter    func(*types.PortaskMessage) bool
	processor MessageProcessor
}

// NewFilterMessageProcessor creates a filter message processor
func NewFilterMessageProcessor(
	filter func(*types.PortaskMessage) bool,
	processor MessageProcessor,
) *FilterMessageProcessor {
	return &FilterMessageProcessor{
		filter:    filter,
		processor: processor,
	}
}

// ProcessMessage processes a message if it passes the filter
func (p *FilterMessageProcessor) ProcessMessage(ctx context.Context, message *types.PortaskMessage) error {
	if p.filter(message) {
		return p.processor.ProcessMessage(ctx, message)
	}
	return nil // Skip message
}

// RetryMessageProcessor implements message processing with retries
type RetryMessageProcessor struct {
	processor  MessageProcessor
	maxRetries int
	backoff    time.Duration
}

// NewRetryMessageProcessor creates a retry message processor
func NewRetryMessageProcessor(processor MessageProcessor, maxRetries int, backoff time.Duration) *RetryMessageProcessor {
	return &RetryMessageProcessor{
		processor:  processor,
		maxRetries: maxRetries,
		backoff:    backoff,
	}
}

// ProcessMessage processes a message with retries
func (p *RetryMessageProcessor) ProcessMessage(ctx context.Context, message *types.PortaskMessage) error {
	var lastErr error

	for retry := 0; retry <= p.maxRetries; retry++ {
		if retry > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.backoff):
			}
		}

		if err := p.processor.ProcessMessage(ctx, message); err == nil {
			return nil // Success
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("failed after %d retries: %w", p.maxRetries, lastErr)
}
