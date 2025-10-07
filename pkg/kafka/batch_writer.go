package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/meftunca/portask/pkg/storage/dragonfly"
	"github.com/meftunca/portask/pkg/types"
)

// BatchWriter accumulates messages and writes them in batches to improve throughput
type BatchWriter struct {
	store         *dragonfly.DragonflyStore
	ctx           context.Context
	batchSize     int           // Maximum number of messages before flush
	flushInterval time.Duration // Maximum time to wait before flush
	buffer        []*types.PortaskMessage
	mu            sync.Mutex
	closeCh       chan struct{}
	wg            sync.WaitGroup
	messageCount  int64 // Total messages written
}

// BatchWriterConfig configures the batch writer
type BatchWriterConfig struct {
	Store         *dragonfly.DragonflyStore
	Ctx           context.Context
	BatchSize     int           // Default: 1000
	FlushInterval time.Duration // Default: 10ms
}

// NewBatchWriter creates a new batch writer
func NewBatchWriter(config *BatchWriterConfig) *BatchWriter {
	if config.BatchSize == 0 {
		config.BatchSize = 1000 // Default batch size
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 10 * time.Millisecond // Default flush interval
	}

	bw := &BatchWriter{
		store:         config.Store,
		ctx:           config.Ctx,
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		buffer:        make([]*types.PortaskMessage, 0, config.BatchSize),
		closeCh:       make(chan struct{}),
	}

	// Start the background flush goroutine
	bw.wg.Add(1)
	go bw.flushLoop()

	return bw
}

// Write adds a message to the batch buffer
func (bw *BatchWriter) Write(msg *types.PortaskMessage) error {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	bw.buffer = append(bw.buffer, msg)

	// Flush if batch size reached
	if len(bw.buffer) >= bw.batchSize {
		return bw.flushLocked()
	}

	return nil
}

// flushLoop periodically flushes the buffer
func (bw *BatchWriter) flushLoop() {
	defer bw.wg.Done()
	ticker := time.NewTicker(bw.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bw.mu.Lock()
			if len(bw.buffer) > 0 {
				_ = bw.flushLocked() // Ignore errors in background flush
			}
			bw.mu.Unlock()

		case <-bw.closeCh:
			// Final flush before closing
			bw.mu.Lock()
			if len(bw.buffer) > 0 {
				_ = bw.flushLocked()
			}
			bw.mu.Unlock()
			return
		}
	}
}

// flushLocked flushes the current buffer (must hold lock)
func (bw *BatchWriter) flushLocked() error {
	if len(bw.buffer) == 0 {
		return nil
	}

	// Use Dragonfly's StoreBatch for efficient batch writing
	batch := &types.MessageBatch{
		Messages: bw.buffer,
	}

	err := bw.store.StoreBatch(bw.ctx, batch)
	if err != nil {
		return err
	}

	// Update counter
	bw.messageCount += int64(len(bw.buffer))

	// Clear buffer
	bw.buffer = bw.buffer[:0]

	return nil
}

// Flush manually flushes the buffer
func (bw *BatchWriter) Flush() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.flushLocked()
}

// Close stops the batch writer and flushes remaining messages
func (bw *BatchWriter) Close() error {
	close(bw.closeCh)
	bw.wg.Wait()
	return nil
}

// Stats returns statistics about the batch writer
func (bw *BatchWriter) Stats() int64 {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.messageCount
}

