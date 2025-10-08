package dragonfly

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

// StoreBatchParallel stores messages in parallel using multiple connections from the pool
// Each sub-batch uses a separate connection for maximum throughput
func (d *DragonflyStore) StoreBatchParallel(ctx context.Context, batch *types.MessageBatch, subBatchSize int) error {
	start := time.Now()
	defer func() {
		d.updateResponseTime(time.Since(start))
		d.metrics.TotalOperations++
	}()

	if batch == nil || len(batch.Messages) == 0 {
		return nil
	}

	// Default sub-batch size
	if subBatchSize <= 0 {
		subBatchSize = 50 // Optimal: 50 messages per connection
	}

	messages := batch.Messages
	totalMessages := len(messages)

	// Calculate number of sub-batches
	numSubBatches := (totalMessages + subBatchSize - 1) / subBatchSize

	// Error tracking
	var (
		wg       sync.WaitGroup
		errMutex sync.Mutex
		firstErr error
	)

	// Process sub-batches in parallel
	for i := 0; i < numSubBatches; i++ {
		start := i * subBatchSize
		end := start + subBatchSize
		if end > totalMessages {
			end = totalMessages
		}

		subBatch := messages[start:end]

		wg.Add(1)
		go func(msgs []*types.PortaskMessage) {
			defer wg.Done()

			// Each goroutine gets its own pipeline (and thus its own connection from pool)
			pipe := d.client.Pipeline()

			for _, message := range msgs {
				// Ensure maps are initialized (prevent nil pointer during serialization)
				if message.Metadata == nil {
					message.Metadata = make(map[string]string)
				}
				if message.Headers == nil {
					message.Headers = make(map[string]interface{})
				}
				
				// Serialize
				data, err := d.serializer.Serialize(message)
				if err != nil {
					errMutex.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("serialization failed for message %s: %w", message.ID, err)
					}
					errMutex.Unlock()
					return
				}

				// Compress if needed
				if d.config.EnableCompression && len(data) > 1024 {
					data, err = d.compressor.Compress(data)
					if err != nil {
						errMutex.Lock()
						if firstErr == nil {
							firstErr = fmt.Errorf("compression failed for message %s: %w", message.ID, err)
						}
						errMutex.Unlock()
						return
					}
				}

				// Key generation
				key := d.messagePrefix + string(message.ID)

				// TTL
				var ttl time.Duration
				if message.TTL > 0 {
					ttl = time.Duration(message.TTL) * time.Second
				}

				// Add to this goroutine's pipeline
				pipe.Set(ctx, key, data, ttl)
			}

			// Execute this sub-batch's pipeline
			// This will use a connection from the pool
			_, err := pipe.Exec(ctx)
			if err != nil {
				errMutex.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("sub-batch exec failed: %w", err)
				}
				errMutex.Unlock()
			}
		}(subBatch)
	}

	// Wait for all sub-batches to complete
	wg.Wait()

	if firstErr != nil {
		d.metrics.FailedOperations++
		return firstErr
	}

	d.metrics.SuccessfulOperations++
	return nil
}

// ParallelBatchConfig holds configuration for parallel batch writes
type ParallelBatchConfig struct {
	SubBatchSize int // Messages per sub-batch (default: 50)
	MaxParallel  int // Max concurrent sub-batches (0 = unlimited)
}

// StoreBatchParallelWithConfig provides fine-grained control over parallel batching
func (d *DragonflyStore) StoreBatchParallelWithConfig(ctx context.Context, batch *types.MessageBatch, config *ParallelBatchConfig) error {
	if config == nil {
		config = &ParallelBatchConfig{
			SubBatchSize: 50,
			MaxParallel:  0, // Unlimited
		}
	}

	// If MaxParallel is set, use a semaphore to limit concurrency
	if config.MaxParallel > 0 {
		return d.storeBatchWithSemaphore(ctx, batch, config)
	}

	return d.StoreBatchParallel(ctx, batch, config.SubBatchSize)
}

// storeBatchWithSemaphore limits the number of concurrent sub-batches
func (d *DragonflyStore) storeBatchWithSemaphore(ctx context.Context, batch *types.MessageBatch, config *ParallelBatchConfig) error {
	start := time.Now()
	defer func() {
		d.updateResponseTime(time.Since(start))
		d.metrics.TotalOperations++
	}()

	if batch == nil || len(batch.Messages) == 0 {
		return nil
	}

	messages := batch.Messages
	totalMessages := len(messages)
	subBatchSize := config.SubBatchSize
	if subBatchSize <= 0 {
		subBatchSize = 50
	}

	numSubBatches := (totalMessages + subBatchSize - 1) / subBatchSize

	// Semaphore to limit concurrency
	sem := make(chan struct{}, config.MaxParallel)

	var (
		wg       sync.WaitGroup
		errMutex sync.Mutex
		firstErr error
	)

	for i := 0; i < numSubBatches; i++ {
		start := i * subBatchSize
		end := start + subBatchSize
		if end > totalMessages {
			end = totalMessages
		}

		subBatch := messages[start:end]

		wg.Add(1)
		go func(msgs []*types.PortaskMessage) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Process sub-batch
			pipe := d.client.Pipeline()

			for _, message := range msgs {
				// Ensure maps are initialized
				if message.Metadata == nil {
					message.Metadata = make(map[string]string)
				}
				if message.Headers == nil {
					message.Headers = make(map[string]interface{})
				}
				
				data, err := d.serializer.Serialize(message)
				if err != nil {
					errMutex.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("serialization failed: %w", err)
					}
					errMutex.Unlock()
					return
				}

				if d.config.EnableCompression && len(data) > 1024 {
					data, err = d.compressor.Compress(data)
					if err != nil {
						errMutex.Lock()
						if firstErr == nil {
							firstErr = fmt.Errorf("compression failed: %w", err)
						}
						errMutex.Unlock()
						return
					}
				}

				key := d.messagePrefix + string(message.ID)
				var ttl time.Duration
				if message.TTL > 0 {
					ttl = time.Duration(message.TTL) * time.Second
				}

				pipe.Set(ctx, key, data, ttl)
			}

			_, err := pipe.Exec(ctx)
			if err != nil {
				errMutex.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("exec failed: %w", err)
				}
				errMutex.Unlock()
			}
		}(subBatch)
	}

	wg.Wait()

	if firstErr != nil {
		d.metrics.FailedOperations++
		return firstErr
	}

	d.metrics.SuccessfulOperations++
	return nil
}

