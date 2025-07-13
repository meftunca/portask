package processor

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

// Errors
var (
	ErrBackpressure        = errors.New("processor backpressure detected")
	ErrProcessingPanic     = errors.New("processing panic occurred")
	ErrUnknownProcessType  = errors.New("unknown process type")
	ErrNilMessage          = errors.New("message is nil")
	ErrEmptyTopic          = errors.New("topic is empty")
	ErrNilValue            = errors.New("message value is nil")
	ErrCompressionFailed   = errors.New("compression failed")
	ErrDecompressionFailed = errors.New("decompression failed")
)

// ProcessorMetrics holds processor performance metrics
type ProcessorMetrics struct {
	TaskCounter        uint64
	TotalTasks         uint64
	SuccessCount       uint64
	ErrorCount         uint64
	BackpressureCount  uint64
	CompressedMessages uint64
	TotalLatency       time.Duration
	TotalBytesIn       int64
	TotalBytesOut      int64
	AvgLatency         time.Duration
	Throughput         float64
	CompressionRatio   float64
	mu                 sync.RWMutex
}

// NewProcessorMetrics creates new processor metrics
func NewProcessorMetrics() *ProcessorMetrics {
	return &ProcessorMetrics{}
}

// GetAvgLatency calculates average latency
func (pm *ProcessorMetrics) GetAvgLatency() time.Duration {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.TotalTasks == 0 {
		return 0
	}
	return time.Duration(int64(pm.TotalLatency) / int64(pm.TotalTasks))
}

// GetThroughput calculates throughput (messages per second)
func (pm *ProcessorMetrics) GetThroughput() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.TotalLatency == 0 {
		return 0
	}
	return float64(pm.TotalTasks) / pm.TotalLatency.Seconds()
}

// GetCompressionRatio calculates compression ratio
func (pm *ProcessorMetrics) GetCompressionRatio() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.TotalBytesIn == 0 {
		return 0
	}
	return float64(pm.TotalBytesOut) / float64(pm.TotalBytesIn)
}

// GetSuccessRate calculates success rate
func (pm *ProcessorMetrics) GetSuccessRate() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.TotalTasks == 0 {
		return 0
	}
	return float64(pm.SuccessCount) / float64(pm.TotalTasks)
}

// WorkerMetrics holds worker performance metrics
type WorkerMetrics struct {
	TasksProcessed uint64
	SuccessCount   uint64
	ErrorCount     uint64
	TotalLatency   time.Duration
	LastActivity   time.Time
	mu             sync.RWMutex
}

// NewWorkerMetrics creates new worker metrics
func NewWorkerMetrics() *WorkerMetrics {
	return &WorkerMetrics{
		LastActivity: time.Now(),
	}
}

// GetAvgLatency calculates average latency for worker
func (wm *WorkerMetrics) GetAvgLatency() time.Duration {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	if wm.TasksProcessed == 0 {
		return 0
	}
	return time.Duration(int64(wm.TotalLatency) / int64(wm.TasksProcessed))
}

// CompressionPool manages compression algorithm instances
type CompressionPool struct {
	algorithm string
	pool      sync.Pool
}

// NewCompressionPool creates a new compression pool
func NewCompressionPool(algorithm string) *CompressionPool {
	cp := &CompressionPool{
		algorithm: algorithm,
	}

	cp.pool = sync.Pool{
		New: func() interface{} {
			return NewCompressor(algorithm)
		},
	}

	return cp
}

// Get retrieves a compressor from the pool
func (cp *CompressionPool) Get() Compressor {
	return cp.pool.Get().(Compressor)
}

// Put returns a compressor to the pool
func (cp *CompressionPool) Put(compressor Compressor) {
	compressor.Reset()
	cp.pool.Put(compressor)
}

// Compressor interface for compression algorithms
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Reset()
	Algorithm() string
}

// ZstdCompressor implements Zstandard compression
type ZstdCompressor struct {
	level int
}

// NewCompressor creates a new compressor for the specified algorithm
func NewCompressor(algorithm string) Compressor {
	switch algorithm {
	case "zstd":
		return &ZstdCompressor{level: 6}
	case "lz4":
		return &LZ4Compressor{}
	case "snappy":
		return &SnappyCompressor{}
	default:
		return &NoOpCompressor{}
	}
}

// Compress compresses data using Zstandard
func (zc *ZstdCompressor) Compress(data []byte) ([]byte, error) {
	// Placeholder - would use actual zstd library
	// For now, return original data
	compressed := make([]byte, len(data))
	copy(compressed, data)
	return compressed, nil
}

// Decompress decompresses data using Zstandard
func (zc *ZstdCompressor) Decompress(data []byte) ([]byte, error) {
	// Placeholder - would use actual zstd library
	decompressed := make([]byte, len(data))
	copy(decompressed, data)
	return decompressed, nil
}

// Reset resets the compressor state
func (zc *ZstdCompressor) Reset() {
	// Reset compressor state if needed
}

// Algorithm returns the compression algorithm name
func (zc *ZstdCompressor) Algorithm() string {
	return "zstd"
}

// LZ4Compressor implements LZ4 compression
type LZ4Compressor struct{}

func (lc *LZ4Compressor) Compress(data []byte) ([]byte, error) {
	// Placeholder implementation
	compressed := make([]byte, len(data))
	copy(compressed, data)
	return compressed, nil
}

func (lc *LZ4Compressor) Decompress(data []byte) ([]byte, error) {
	decompressed := make([]byte, len(data))
	copy(decompressed, data)
	return decompressed, nil
}

func (lc *LZ4Compressor) Reset() {}

func (lc *LZ4Compressor) Algorithm() string {
	return "lz4"
}

// SnappyCompressor implements Snappy compression
type SnappyCompressor struct{}

func (sc *SnappyCompressor) Compress(data []byte) ([]byte, error) {
	// Placeholder implementation
	compressed := make([]byte, len(data))
	copy(compressed, data)
	return compressed, nil
}

func (sc *SnappyCompressor) Decompress(data []byte) ([]byte, error) {
	decompressed := make([]byte, len(data))
	copy(decompressed, data)
	return decompressed, nil
}

func (sc *SnappyCompressor) Reset() {}

func (sc *SnappyCompressor) Algorithm() string {
	return "snappy"
}

// NoOpCompressor implements no compression
type NoOpCompressor struct{}

func (nc *NoOpCompressor) Compress(data []byte) ([]byte, error) {
	return data, nil
}

func (nc *NoOpCompressor) Decompress(data []byte) ([]byte, error) {
	return data, nil
}

func (nc *NoOpCompressor) Reset() {}

func (nc *NoOpCompressor) Algorithm() string {
	return "none"
}

// BatchProcessor processes messages in batches for higher throughput
type BatchProcessor struct {
	processor *MessageProcessor
	batchSize int
	timeout   time.Duration
	buffer    []*ProcessTask
	mu        sync.Mutex
	ticker    *time.Ticker
	stopCh    chan struct{}
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(processor *MessageProcessor, batchSize int, timeout time.Duration) *BatchProcessor {
	bp := &BatchProcessor{
		processor: processor,
		batchSize: batchSize,
		timeout:   timeout,
		buffer:    make([]*ProcessTask, 0, batchSize),
		ticker:    time.NewTicker(timeout),
		stopCh:    make(chan struct{}),
	}

	go bp.flushLoop()
	return bp
}

// AddTask adds a task to the batch buffer
func (bp *BatchProcessor) AddTask(task *ProcessTask) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.buffer = append(bp.buffer, task)

	if len(bp.buffer) >= bp.batchSize {
		bp.flush()
	}
}

// flush processes the current batch
func (bp *BatchProcessor) flush() {
	if len(bp.buffer) == 0 {
		return
	}

	// Create batch task
	batchTask := &ProcessTask{
		ID:        atomic.AddUint64(&bp.processor.metrics.TaskCounter, 1),
		Type:      ProcessTypeBatch,
		StartTime: time.Now(),
	}

	// Collect all messages
	messages := make([]*types.PortaskMessage, 0, len(bp.buffer))
	for _, task := range bp.buffer {
		if task.Message != nil {
			messages = append(messages, task.Message)
		}
	}
	batchTask.Batch = messages

	// Submit batch
	select {
	case bp.processor.processingQueue <- batchTask:
	default:
		// Queue full, handle overflow
		bp.processor.metrics.BackpressureCount++
	}

	// Clear buffer
	bp.buffer = bp.buffer[:0]
}

// flushLoop periodically flushes the buffer
func (bp *BatchProcessor) flushLoop() {
	for {
		select {
		case <-bp.ticker.C:
			bp.mu.Lock()
			bp.flush()
			bp.mu.Unlock()
		case <-bp.stopCh:
			return
		}
	}
}

// Stop stops the batch processor
func (bp *BatchProcessor) Stop() {
	close(bp.stopCh)
	bp.ticker.Stop()

	bp.mu.Lock()
	bp.flush() // Final flush
	bp.mu.Unlock()
}
