package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

// BatchWriter accumulates messages and writes them in batches for better throughput
type BatchWriter struct {
	storage       StorageBackend
	config        *BatchWriterConfig
	buffer        []*types.PortaskMessage
	mu            sync.Mutex
	flushTimer    *time.Ticker
	stopCh        chan struct{}
	wg            sync.WaitGroup
	messageCount  int64
	batchCount    int64
	totalLatency  time.Duration
	running       bool
}

// BatchWriterConfig configures the batch writer
type BatchWriterConfig struct {
	FlushInterval time.Duration // Time to wait before flushing (e.g., 10ms)
	BatchSize     int           // Number of messages to accumulate before flushing (e.g., 1000)
	MaxRetries    int           // Max retries for failed batches
}

// StorageBackend defines the interface for storage operations
type StorageBackend interface {
	StoreBatch(ctx context.Context, batch *types.MessageBatch) error
}

// DefaultBatchWriterConfig returns default configuration
func DefaultBatchWriterConfig() *BatchWriterConfig {
	return &BatchWriterConfig{
		FlushInterval: 10 * time.Millisecond, // 10ms flush interval
		BatchSize:     1000,                   // 1000 messages per batch
		MaxRetries:    3,
	}
}

// NewBatchWriter creates a new batch writer
func NewBatchWriter(storage StorageBackend, config *BatchWriterConfig) *BatchWriter {
	if config == nil {
		config = DefaultBatchWriterConfig()
	}
	
	return &BatchWriter{
		storage:      storage,
		config:       config,
		buffer:       make([]*types.PortaskMessage, 0, config.BatchSize),
		flushTimer:   time.NewTicker(config.FlushInterval),
		stopCh:       make(chan struct{}),
	}
}

// Start starts the batch writer
func (bw *BatchWriter) Start(ctx context.Context) error {
	bw.mu.Lock()
	if bw.running {
		bw.mu.Unlock()
		return fmt.Errorf("batch writer already running")
	}
	bw.running = true
	bw.mu.Unlock()
	
	bw.wg.Add(1)
	go bw.flushLoop(ctx)
	
	return nil
}

// Stop stops the batch writer and flushes remaining messages
func (bw *BatchWriter) Stop() error {
	bw.mu.Lock()
	if !bw.running {
		bw.mu.Unlock()
		return nil
	}
	bw.running = false
	bw.mu.Unlock()
	
	close(bw.stopCh)
	bw.wg.Wait()
	
	// Final flush
	return bw.flush(context.Background())
}

// Write adds a message to the batch
func (bw *BatchWriter) Write(msg *types.PortaskMessage) error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	
	bw.buffer = append(bw.buffer, msg)
	bw.messageCount++
	
	// Flush if batch size reached
	if len(bw.buffer) >= bw.config.BatchSize {
		return bw.flushLocked(context.Background())
	}
	
	return nil
}

// flushLoop periodically flushes the buffer
func (bw *BatchWriter) flushLoop(ctx context.Context) {
	defer bw.wg.Done()
	
	for {
		select {
		case <-bw.flushTimer.C:
			bw.mu.Lock()
			if len(bw.buffer) > 0 {
				bw.flushLocked(ctx)
			}
			bw.mu.Unlock()
			
		case <-bw.stopCh:
			return
			
		case <-ctx.Done():
			return
		}
	}
}

// flush writes the current buffer to storage (with lock)
func (bw *BatchWriter) flush(ctx context.Context) error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.flushLocked(ctx)
}

// flushLocked writes the current buffer to storage (caller must hold lock)
func (bw *BatchWriter) flushLocked(ctx context.Context) error {
	if len(bw.buffer) == 0 {
		return nil
	}
	
	start := time.Now()
	
	// Create batch
	batch := &types.MessageBatch{
		Messages: bw.buffer,
	}
	
	// Write to storage with retries
	var err error
	for attempt := 0; attempt <= bw.config.MaxRetries; attempt++ {
		err = bw.storage.StoreBatch(ctx, batch)
		if err == nil {
			break
		}
		
		if attempt < bw.config.MaxRetries {
			// Exponential backoff
			time.Sleep(time.Duration(1<<uint(attempt)) * 10 * time.Millisecond)
		}
	}
	
	if err != nil {
		return fmt.Errorf("batch write failed after %d retries: %w", bw.config.MaxRetries, err)
	}
	
	// Update metrics
	bw.batchCount++
	bw.totalLatency += time.Since(start)
	
	// Clear buffer
	bw.buffer = bw.buffer[:0]
	
	return nil
}

// GetStats returns batch writer statistics
func (bw *BatchWriter) GetStats() BatchWriterStats {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	
	avgLatency := time.Duration(0)
	if bw.batchCount > 0 {
		avgLatency = bw.totalLatency / time.Duration(bw.batchCount)
	}
	
	return BatchWriterStats{
		TotalMessages:  bw.messageCount,
		TotalBatches:   bw.batchCount,
		BufferedCount:  int64(len(bw.buffer)),
		AvgBatchSize:   float64(bw.messageCount) / float64(bw.batchCount),
		AvgLatency:     avgLatency,
		FlushInterval:  bw.config.FlushInterval,
		BatchSizeLimit: bw.config.BatchSize,
	}
}

// BatchWriterStats holds batch writer statistics
type BatchWriterStats struct {
	TotalMessages  int64
	TotalBatches   int64
	BufferedCount  int64
	AvgBatchSize   float64
	AvgLatency     time.Duration
	FlushInterval  time.Duration
	BatchSizeLimit int
}

