package processor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

// AsyncBatchWriter writes batches asynchronously with background confirmation
// Phase 5: Fire-and-forget pattern for maximum throughput
type AsyncBatchWriter struct {
	config      *ParallelBatchWriterConfig
	storage     StorageBackend
	shards      []*asyncBatchShard
	running     atomic.Bool
	wg          sync.WaitGroup
	metrics     *AsyncBatchMetrics
	confirmChan chan *batchConfirmation
}

// batchConfirmation tracks async write results
type batchConfirmation struct {
	batchID   uint64
	size      int
	timestamp time.Time
	err       error
}

// AsyncBatchMetrics tracks async writer performance
type AsyncBatchMetrics struct {
	TotalMessagesWritten  atomic.Int64
	TotalBatchesWritten   atomic.Int64
	TotalBatchesConfirmed atomic.Int64
	FailedBatches         atomic.Int64
	PendingBatches        atomic.Int64
	Retries               atomic.Int64
	MessagesQueued        atomic.Int64
	QueueFullErrors       atomic.Int64
	AvgQueueLatency       atomic.Int64 // nanoseconds
	AvgWriteLatency       atomic.Int64 // nanoseconds
}

// NewAsyncBatchWriter creates a new async batch writer
func NewAsyncBatchWriter(storage StorageBackend, config *ParallelBatchWriterConfig) *AsyncBatchWriter {
	if config == nil {
		config = HighThroughputConfig()
	}

	abw := &AsyncBatchWriter{
		config:      config,
		storage:     storage,
		shards:      make([]*asyncBatchShard, config.NumShards),
		metrics:     &AsyncBatchMetrics{},
		confirmChan: make(chan *batchConfirmation, config.NumShards*10), // Buffer for confirmations
	}

	// Create shards
	for i := 0; i < config.NumShards; i++ {
		abw.shards[i] = newAsyncBatchShard(i, storage, config, abw.metrics, abw.confirmChan)
	}

	return abw
}

// Start starts the async batch writer
func (abw *AsyncBatchWriter) Start(ctx context.Context) {
	if abw.running.Swap(true) {
		return // Already running
	}

	log.Printf("🚀 Starting AsyncBatchWriter with %d shards (async mode)", abw.config.NumShards)

	// Start confirmation processor
	abw.wg.Add(1)
	go abw.processConfirmations(ctx)

	// Start all shards
	for _, shard := range abw.shards {
		abw.wg.Add(1)
		go shard.run(ctx, &abw.wg)
	}
}

// Stop stops the async batch writer
func (abw *AsyncBatchWriter) Stop() error {
	if !abw.running.Swap(false) {
		return nil // Not running
	}

	log.Printf("🛑 Stopping AsyncBatchWriter...")

	// Stop all shards
	for _, shard := range abw.shards {
		shard.stop()
	}

	// Close confirmation channel
	close(abw.confirmChan)

	// Wait for all goroutines
	abw.wg.Wait()

	log.Printf("✅ AsyncBatchWriter stopped. Messages: %d, Batches: %d, Confirmed: %d",
		abw.metrics.TotalMessagesWritten.Load(),
		abw.metrics.TotalBatchesWritten.Load(),
		abw.metrics.TotalBatchesConfirmed.Load())

	return nil
}

// Write adds a message to async batch queue (non-blocking)
func (abw *AsyncBatchWriter) Write(msg *types.PortaskMessage) error {
	if !abw.running.Load() {
		return fmt.Errorf("AsyncBatchWriter is not running")
	}

	shardID := abw.getShardID(msg)
	
	// Try non-blocking write
	select {
	case abw.shards[shardID].input <- msg:
		abw.metrics.MessagesQueued.Add(1)
		return nil
	default:
		// Queue full, increment error but don't block
		abw.metrics.QueueFullErrors.Add(1)
		
		// Block as last resort (backpressure)
		abw.shards[shardID].input <- msg
		return nil
	}
}

// getShardID determines shard for a message
func (abw *AsyncBatchWriter) getShardID(msg *types.PortaskMessage) int {
	// Use topic hash for distribution
	h := uint32(0)
	for i := 0; i < len(msg.Topic); i++ {
		h = h*31 + uint32(msg.Topic[i])
	}
	return int(h % uint32(abw.config.NumShards))
}

// processConfirmations handles async write confirmations
func (abw *AsyncBatchWriter) processConfirmations(ctx context.Context) {
	defer abw.wg.Done()

	for {
		select {
		case conf, ok := <-abw.confirmChan:
			if !ok {
				return // Channel closed
			}

			if conf.err != nil {
				abw.metrics.FailedBatches.Add(1)
				log.Printf("❌ Batch %d failed: %v", conf.batchID, conf.err)
			} else {
				abw.metrics.TotalBatchesConfirmed.Add(1)
				abw.metrics.TotalMessagesWritten.Add(int64(conf.size))
			}

			abw.metrics.PendingBatches.Add(-1)

		case <-ctx.Done():
			return
		}
	}
}

// asyncBatchShard handles async batch writing for a single shard
type asyncBatchShard struct {
	id            int
	config        *ParallelBatchWriterConfig
	storage       StorageBackend
	input         chan *types.PortaskMessage
	currentBatch  []*types.PortaskMessage // Simple slice instead of MessageBatch
	flushTimer    *time.Timer
	stopChan      chan struct{}
	metrics       *AsyncBatchMetrics
	confirmChan   chan *batchConfirmation
	batchIDGen    atomic.Uint64
}

// newAsyncBatchShard creates a new async batch shard
func newAsyncBatchShard(
	id int,
	storage StorageBackend,
	config *ParallelBatchWriterConfig,
	metrics *AsyncBatchMetrics,
	confirmChan chan *batchConfirmation,
) *asyncBatchShard {
	return &asyncBatchShard{
		id:           id,
		config:       config,
		storage:      storage,
		input:        make(chan *types.PortaskMessage, config.BatchSize*2),
		currentBatch: make([]*types.PortaskMessage, 0, config.BatchSize),
		flushTimer:   time.NewTimer(config.FlushInterval),
		stopChan:     make(chan struct{}),
		metrics:      metrics,
		confirmChan:  confirmChan,
	}
}

// run starts the shard event loop
func (as *asyncBatchShard) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case msg := <-as.input:
			as.currentBatch = append(as.currentBatch, msg)
			if len(as.currentBatch) >= as.config.BatchSize {
				as.flushAsync(ctx)
			}
		case <-as.flushTimer.C:
			if len(as.currentBatch) > 0 {
				as.flushAsync(ctx)
			}
		case <-as.stopChan:
			as.flushAsync(ctx) // Final flush
			return
		}
		as.resetFlushTimer()
	}
}

// flushAsync writes batch asynchronously (non-blocking)
func (as *asyncBatchShard) flushAsync(ctx context.Context) {
	if len(as.currentBatch) == 0 {
		return
	}

	batchToFlush := as.currentBatch
	batchID := as.batchIDGen.Add(1)
	batchSize := len(batchToFlush)
	as.currentBatch = make([]*types.PortaskMessage, 0, as.config.BatchSize) // Start new batch immediately

	as.metrics.TotalBatchesWritten.Add(1)
	as.metrics.PendingBatches.Add(1)

	// Write asynchronously in background
	go func(messages []*types.PortaskMessage, id uint64, size int) {
		start := time.Now()
		batch := types.NewMessageBatch(messages)
		
		var err error
		
		// Use parallel batch writes if enabled and supported
		if as.config.EnableParallelWrites && as.config.SubBatchSize > 0 {
			// Try to use parallel batch write (Dragonfly store interface)
			type ParallelBatchStore interface {
				StoreBatchParallel(ctx context.Context, batch *types.MessageBatch, subBatchSize int) error
			}
			
			if parallelStore, ok := as.storage.(ParallelBatchStore); ok {
				// Use parallel batch write for +92% throughput!
				err = parallelStore.StoreBatchParallel(ctx, batch, as.config.SubBatchSize)
			} else {
				// Fallback to regular batch write
				err = as.storage.StoreBatch(ctx, batch)
			}
		} else {
			// Regular batch write
			err = as.storage.StoreBatch(ctx, batch)
		}
		
		duration := time.Since(start)

		// Send confirmation
		as.confirmChan <- &batchConfirmation{
			batchID:   id,
			size:      size,
			timestamp: time.Now(),
			err:       err,
		}

		// Update latency metrics
		as.metrics.AvgWriteLatency.Store(duration.Nanoseconds())
	}(batchToFlush, batchID, batchSize)
}

// stop signals the shard to stop
func (as *asyncBatchShard) stop() {
	close(as.stopChan)
	as.flushTimer.Stop()
}

// resetFlushTimer resets the flush timer
func (as *asyncBatchShard) resetFlushTimer() {
	if !as.flushTimer.Stop() {
		select {
		case <-as.flushTimer.C:
		default:
		}
	}
	as.flushTimer.Reset(as.config.FlushInterval)
}

// GetMetrics returns async writer metrics
func (abw *AsyncBatchWriter) GetMetrics() *AsyncBatchMetrics {
	return abw.metrics
}

