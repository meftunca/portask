package processor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/meftunca/portask/pkg/memory"
	"github.com/meftunca/portask/pkg/types"
)

// MessageProcessor handles high-performance message processing
type MessageProcessor struct {
	config          *ProcessorConfig
	messagePool     *memory.MessagePool
	bufferPool      *memory.BufferPool
	compressionPool *CompressionPool
	processingQueue chan *ProcessTask
	resultQueue     chan *ProcessResult
	workers         []*Worker
	metrics         *ProcessorMetrics
	running         int32
	mu              sync.RWMutex
	events          []ProcessorEvent
	eventsMu        sync.Mutex
	workerHealth    map[int]*WorkerHealth
}

// ProcessorConfig configures the message processor
type ProcessorConfig struct {
	WorkerCount           int
	QueueSize             int
	BatchSize             int
	EnableCompression     bool
	CompressionLevel      int
	CompressionAlgorithm  string
	EnableBackpressure    bool
	BackpressureThreshold float64
	ProcessingTimeout     time.Duration
	EnableSIMD            bool
	EnableZeroCopy        bool
}

// DefaultProcessorConfig returns default processor configuration
func DefaultProcessorConfig() *ProcessorConfig {
	return &ProcessorConfig{
		WorkerCount:           8,
		QueueSize:             10000,
		BatchSize:             100,
		EnableCompression:     true,
		CompressionLevel:      6,
		CompressionAlgorithm:  "zstd",
		EnableBackpressure:    true,
		BackpressureThreshold: 0.8,
		ProcessingTimeout:     time.Second * 30,
		EnableSIMD:            true,
		EnableZeroCopy:        true,
	}
}

// ProcessTask represents a processing task
type ProcessTask struct {
	ID        uint64
	Message   *types.PortaskMessage
	Batch     []*types.PortaskMessage
	Type      ProcessType
	Context   context.Context
	StartTime time.Time
	Callback  func(*ProcessResult)
}

// ProcessResult represents processing result
type ProcessResult struct {
	TaskID     uint64
	Success    bool
	Error      error
	Message    *types.PortaskMessage
	Batch      []*types.PortaskMessage
	Latency    time.Duration
	BytesIn    int64
	BytesOut   int64
	Compressed bool
}

// ProcessType defines the type of processing
type ProcessType int

const (
	ProcessTypeSingle ProcessType = iota
	ProcessTypeBatch
	ProcessTypeCompress
	ProcessTypeDecompress
)

// ProcessorEvent represents a diagnostic or lifecycle event in the processor
// Type: "worker_start", "worker_stop", "panic", "error", "batch", "backpressure", "custom"
type ProcessorEvent struct {
	Time     time.Time
	Type     string
	WorkerID int
	Message  string
	TaskID   uint64
	Error    error
}

// WorkerHealth holds health and last error for a worker
type WorkerHealth struct {
	LastError error
	Healthy   bool
	LastEvent time.Time
}

// NewMessageProcessor creates a new high-performance message processor
func NewMessageProcessor(config *ProcessorConfig) *MessageProcessor {
	if config == nil {
		config = DefaultProcessorConfig()
	}

	mp := &MessageProcessor{
		config:          config,
		messagePool:     memory.NewMessagePool(1000),
		bufferPool:      memory.NewBufferPool(),
		compressionPool: NewCompressionPool(config.CompressionAlgorithm),
		processingQueue: make(chan *ProcessTask, config.QueueSize),
		resultQueue:     make(chan *ProcessResult, config.QueueSize),
		workers:         make([]*Worker, config.WorkerCount),
		metrics:         NewProcessorMetrics(),
	}

	// Initialize workers
	for i := 0; i < config.WorkerCount; i++ {
		mp.workers[i] = NewWorker(i, mp)
	}

	return mp
}

// Start starts the message processor
func (mp *MessageProcessor) Start(ctx context.Context) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if atomic.LoadInt32(&mp.running) == 1 {
		return nil
	}

	// Start workers
	for _, worker := range mp.workers {
		go worker.Run(ctx)
	}

	// Start result handler
	go mp.handleResults(ctx)

	atomic.StoreInt32(&mp.running, 1)
	return nil
}

// Stop stops the message processor
func (mp *MessageProcessor) Stop() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if atomic.LoadInt32(&mp.running) == 0 {
		return nil
	}

	atomic.StoreInt32(&mp.running, 0)
	close(mp.processingQueue)
	close(mp.resultQueue)

	return nil
}

// ProcessMessage processes a single message
func (mp *MessageProcessor) ProcessMessage(ctx context.Context, msg *types.PortaskMessage) (*types.PortaskMessage, error) {
	// Create buffered result channel to avoid blocking
	resultChan := make(chan *ProcessResult, 1)

	task := &ProcessTask{
		ID:        atomic.AddUint64(&mp.metrics.TaskCounter, 1),
		Message:   msg,
		Type:      ProcessTypeSingle,
		Context:   ctx,
		StartTime: time.Now(),
		Callback: func(result *ProcessResult) {
			// Always try to send result with timeout
			select {
			case resultChan <- result:
				// Success
			case <-time.After(100 * time.Millisecond):
				// Timeout, result will be lost but won't deadlock
			}
		},
	}

	// Check backpressure
	if mp.config.EnableBackpressure && mp.isBackpressured() {
		mp.metrics.BackpressureCount++
		return nil, fmt.Errorf("backpressure detected")
	}

	// Send task with timeout to avoid blocking
	select {
	case mp.processingQueue <- task:
		// Wait for result with timeout
		select {
		case result := <-resultChan:
			if result.Error != nil {
				return nil, result.Error
			}
			return result.Message, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(mp.config.ProcessingTimeout):
			return nil, fmt.Errorf("processing timeout")
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Second):
		return nil, fmt.Errorf("queue is full")
	}
}

// ProcessBatch processes a batch of messages
func (mp *MessageProcessor) ProcessBatch(ctx context.Context, batch []*types.PortaskMessage) ([]*types.PortaskMessage, error) {
	task := &ProcessTask{
		ID:        atomic.AddUint64(&mp.metrics.TaskCounter, 1),
		Batch:     batch,
		Type:      ProcessTypeBatch,
		Context:   ctx,
		StartTime: time.Now(),
	}

	// Check backpressure
	if mp.config.EnableBackpressure && mp.isBackpressured() {
		mp.metrics.BackpressureCount++
		return nil, ErrBackpressure
	}

	select {
	case mp.processingQueue <- task:
		// Wait for result
		result := <-mp.resultQueue
		if result.Error != nil {
			return nil, result.Error
		}
		return result.Batch, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Worker represents a processing worker
type Worker struct {
	id        int
	processor *MessageProcessor
	metrics   *WorkerMetrics
}

// NewWorker creates a new worker
func NewWorker(id int, processor *MessageProcessor) *Worker {
	return &Worker{
		id:        id,
		processor: processor,
		metrics:   NewWorkerMetrics(),
	}
}

// Run runs the worker loop
func (w *Worker) Run(ctx context.Context) {
	w.processor.AddEvent(ProcessorEvent{
		Time:     time.Now(),
		Type:     "worker_start",
		WorkerID: w.id,
		Message:  "Worker started",
	})
	for {
		select {
		case task := <-w.processor.processingQueue:
			if task == nil {
				w.processor.AddEvent(ProcessorEvent{
					Time:     time.Now(),
					Type:     "worker_stop",
					WorkerID: w.id,
					Message:  "Worker stopped (queue closed)",
				})
				return // Channel closed
			}
			w.processTask(task)
		case <-ctx.Done():
			w.processor.AddEvent(ProcessorEvent{
				Time:     time.Now(),
				Type:     "worker_stop",
				WorkerID: w.id,
				Message:  "Worker stopped (context cancelled)",
			})
			return
		}
	}
}

// processTask processes a single task
func (w *Worker) processTask(task *ProcessTask) {
	defer func() {
		if r := recover(); r != nil {
			w.processor.AddEvent(ProcessorEvent{
				Time:     time.Now(),
				Type:     "panic",
				WorkerID: w.id,
				TaskID:   task.ID,
				Message:  "Worker panic",
				Error:    ErrProcessingPanic,
			})
			result := &ProcessResult{
				TaskID:  task.ID,
				Success: false,
				Error:   ErrProcessingPanic,
				Latency: time.Since(task.StartTime),
			}
			w.processor.resultQueue <- result
		}
	}()

	start := time.Now()
	w.metrics.TasksProcessed++

	var result *ProcessResult

	switch task.Type {
	case ProcessTypeSingle:
		result = w.processSingleMessage(task)
	case ProcessTypeBatch:
		result = w.processBatchMessages(task)
	case ProcessTypeCompress:
		result = w.compressMessage(task)
	case ProcessTypeDecompress:
		result = w.decompressMessage(task)
	default:
		result = &ProcessResult{
			TaskID:  task.ID,
			Success: false,
			Error:   ErrUnknownProcessType,
			Latency: time.Since(start),
		}
	}

	w.metrics.TotalLatency += result.Latency
	if result.Success {
		w.metrics.SuccessCount++
	} else {
		w.metrics.ErrorCount++
	}

	// Send result via callback if available, otherwise use queue
	if task.Callback != nil {
		task.Callback(result)
	} else {
		w.processor.resultQueue <- result
	}
}

// processSingleMessage processes a single message
func (w *Worker) processSingleMessage(task *ProcessTask) *ProcessResult {
	msg := task.Message
	start := task.StartTime

	// Validate message
	if err := w.validateMessage(msg); err != nil {
		return &ProcessResult{
			TaskID:  task.ID,
			Success: false,
			Error:   err,
			Latency: time.Since(start),
		}
	}

	// Process message - convert to processor's internal format
	processedMsg := w.processor.messagePool.Get()
	if err := w.copyMessage(processedMsg, msg); err != nil {
		w.processor.messagePool.Put(processedMsg)
		return &ProcessResult{
			TaskID:  task.ID,
			Success: false,
			Error:   err,
			Latency: time.Since(start),
		}
	}

	// Apply compression if enabled
	if w.processor.config.EnableCompression {
		if err := w.compressMessageData(processedMsg); err != nil {
			w.processor.messagePool.Put(processedMsg)
			return &ProcessResult{
				TaskID:  task.ID,
				Success: false,
				Error:   err,
				Latency: time.Since(start),
			}
		}
	}

	// Calculate checksums if enabled
	if err := w.calculateChecksum(processedMsg); err != nil {
		w.processor.messagePool.Put(processedMsg)
		return &ProcessResult{
			TaskID:  task.ID,
			Success: false,
			Error:   err,
			Latency: time.Since(start),
		}
	}

	// Convert back to PortaskMessage format
	resultMsg := w.convertToPortaskMessage(processedMsg)
	w.processor.messagePool.Put(processedMsg)

	return &ProcessResult{
		TaskID:     task.ID,
		Success:    true,
		Message:    resultMsg,
		Latency:    time.Since(start),
		BytesIn:    int64(len(msg.Payload)),
		BytesOut:   int64(len(resultMsg.Payload)),
		Compressed: w.processor.config.EnableCompression,
	}
}

// processBatchMessages processes a batch of messages
func (w *Worker) processBatchMessages(task *ProcessTask) *ProcessResult {
	batch := task.Batch
	start := task.StartTime

	processedBatch := make([]*types.PortaskMessage, len(batch))
	totalBytesIn := int64(0)
	totalBytesOut := int64(0)

	// Process each message in the batch
	for i, msg := range batch {
		totalBytesIn += int64(len(msg.Payload))

		// Validate message
		if err := w.validateMessage(msg); err != nil {
			return &ProcessResult{
				TaskID:  task.ID,
				Success: false,
				Error:   err,
				Latency: time.Since(start),
			}
		}

		// Process message
		processedMsg := w.processor.messagePool.Get()
		if err := w.copyMessage(processedMsg, msg); err != nil {
			// Clean up allocated messages
			for j := 0; j < i; j++ {
				// processedBatch[j] is already converted back to PortaskMessage
			}
			w.processor.messagePool.Put(processedMsg)
			return &ProcessResult{
				TaskID:  task.ID,
				Success: false,
				Error:   err,
				Latency: time.Since(start),
			}
		}

		// Apply compression if enabled
		if w.processor.config.EnableCompression {
			if err := w.compressMessageData(processedMsg); err != nil {
				// Clean up allocated messages
				for j := 0; j <= i; j++ {
					// processedBatch[j] is already converted back to PortaskMessage
				}
				w.processor.messagePool.Put(processedMsg)
				return &ProcessResult{
					TaskID:  task.ID,
					Success: false,
					Error:   err,
					Latency: time.Since(start),
				}
			}
		}

		// Calculate checksums if enabled
		if err := w.calculateChecksum(processedMsg); err != nil {
			// Clean up allocated messages
			for j := 0; j <= i; j++ {
				// processedBatch[j] is already converted back to PortaskMessage
			}
			w.processor.messagePool.Put(processedMsg)
			return &ProcessResult{
				TaskID:  task.ID,
				Success: false,
				Error:   err,
				Latency: time.Since(start),
			}
		}

		// Convert back to PortaskMessage and add to batch
		resultMsg := w.convertToPortaskMessage(processedMsg)
		processedBatch[i] = resultMsg
		totalBytesOut += int64(len(resultMsg.Payload))

		// Return memory.Message to pool
		w.processor.messagePool.Put(processedMsg)
	}

	return &ProcessResult{
		TaskID:     task.ID,
		Success:    true,
		Batch:      processedBatch,
		Latency:    time.Since(start),
		BytesIn:    totalBytesIn,
		BytesOut:   totalBytesOut,
		Compressed: w.processor.config.EnableCompression,
	}
}

// compressMessage compresses a message
func (w *Worker) compressMessage(task *ProcessTask) *ProcessResult {
	msg := task.Message
	start := task.StartTime

	compressor := w.processor.compressionPool.Get()
	defer w.processor.compressionPool.Put(compressor)

	compressed, err := compressor.Compress(msg.Payload)
	if err != nil {
		return &ProcessResult{
			TaskID:  task.ID,
			Success: false,
			Error:   err,
			Latency: time.Since(start),
		}
	}

	// Create new message with compressed data
	processedMsg := &types.PortaskMessage{
		ID:        msg.ID,
		Topic:     msg.Topic,
		Payload:   compressed,
		Timestamp: msg.Timestamp,
		Headers:   msg.Headers,
	}

	return &ProcessResult{
		TaskID:     task.ID,
		Success:    true,
		Message:    processedMsg,
		Latency:    time.Since(start),
		BytesIn:    int64(len(msg.Payload)),
		BytesOut:   int64(len(compressed)),
		Compressed: true,
	}
}

// decompressMessage decompresses a message
func (w *Worker) decompressMessage(task *ProcessTask) *ProcessResult {
	msg := task.Message
	start := task.StartTime

	compressor := w.processor.compressionPool.Get()
	defer w.processor.compressionPool.Put(compressor)

	decompressed, err := compressor.Decompress(msg.Payload)
	if err != nil {
		return &ProcessResult{
			TaskID:  task.ID,
			Success: false,
			Error:   err,
			Latency: time.Since(start),
		}
	}

	// Create new message with decompressed data
	processedMsg := &types.PortaskMessage{
		ID:        msg.ID,
		Topic:     msg.Topic,
		Payload:   decompressed,
		Timestamp: msg.Timestamp,
		Headers:   msg.Headers,
	}

	return &ProcessResult{
		TaskID:     task.ID,
		Success:    true,
		Message:    processedMsg,
		Latency:    time.Since(start),
		BytesIn:    int64(len(msg.Payload)),
		BytesOut:   int64(len(decompressed)),
		Compressed: false,
	}
}

// Helper methods

// convertToPortaskMessage converts memory.Message to types.PortaskMessage
func (w *Worker) convertToPortaskMessage(msg *memory.Message) *types.PortaskMessage {
	result := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("%d", msg.ID)),
		Topic:     types.TopicName(msg.Topic),
		Payload:   msg.Value,
		Timestamp: msg.Timestamp,
		Headers:   make(types.MessageHeaders),
	}

	// Convert headers
	for k, v := range msg.Headers {
		result.Headers[k] = string(v)
	}

	return result
}

// copyMessage copies from PortaskMessage to memory.Message
func (w *Worker) copyMessage(dst *memory.Message, src *types.PortaskMessage) error {
	dst.ID = uint64(hash(string(src.ID)))
	dst.Topic = string(src.Topic)
	dst.Timestamp = src.Timestamp

	// Zero-copy if enabled
	if w.processor.config.EnableZeroCopy {
		dst.Value = src.Payload
	} else {
		// Deep copy
		if src.Payload != nil {
			dst.Value = w.processor.bufferPool.Get(len(src.Payload))
			copy(dst.Value, src.Payload)
		}
	}

	// Convert headers
	if src.Headers != nil {
		dst.Headers = make(map[string][]byte)
		for k, v := range src.Headers {
			if str, ok := v.(string); ok {
				dst.Headers[k] = []byte(str)
			}
		}
	}

	return nil
}

// validateMessage validates a PortaskMessage
func (w *Worker) validateMessage(msg *types.PortaskMessage) error {
	if msg == nil {
		return ErrNilMessage
	}
	if len(msg.Topic) == 0 {
		return ErrEmptyTopic
	}
	if msg.Payload == nil {
		return ErrNilValue
	}
	return nil
}

// compressMessageData compresses message data
func (w *Worker) compressMessageData(msg *memory.Message) error {
	if len(msg.Value) == 0 {
		return nil
	}

	compressor := w.processor.compressionPool.Get()
	defer w.processor.compressionPool.Put(compressor)

	compressed, err := compressor.Compress(msg.Value)
	if err != nil {
		return err
	}

	// Only use compressed if it's smaller
	if len(compressed) < len(msg.Value) {
		w.processor.bufferPool.Put(msg.Value)
		msg.Value = compressed
	}

	return nil
}

// calculateChecksum calculates message checksum
func (w *Worker) calculateChecksum(msg *memory.Message) error {
	if w.processor.config.EnableSIMD {
		msg.Checksum = simdCRC32(msg.Value)
	} else {
		msg.Checksum = standardCRC32(msg.Value)
	}
	return nil
}

// hash is a simple hash function for string to uint64 conversion
func hash(s string) uint64 {
	h := uint64(0)
	for _, c := range s {
		h = h*31 + uint64(c)
	}
	return h
}

// isBackpressured checks if the processor is experiencing backpressure
func (mp *MessageProcessor) isBackpressured() bool {
	queueLength := len(mp.processingQueue)
	return float64(queueLength)/float64(mp.config.QueueSize) > mp.config.BackpressureThreshold
}

// handleResults handles processing results
func (mp *MessageProcessor) handleResults(ctx context.Context) {
	for {
		select {
		case result := <-mp.resultQueue:
			if result == nil {
				return // Channel closed
			}
			mp.updateMetrics(result)
		case <-ctx.Done():
			return
		}
	}
}

// updateMetrics updates processor metrics
func (mp *MessageProcessor) updateMetrics(result *ProcessResult) {
	mp.metrics.TotalTasks++
	mp.metrics.TotalLatency += result.Latency
	mp.metrics.TotalBytesIn += result.BytesIn
	mp.metrics.TotalBytesOut += result.BytesOut

	if result.Success {
		mp.metrics.SuccessCount++
	} else {
		mp.metrics.ErrorCount++
	}

	if result.Compressed {
		mp.metrics.CompressedMessages++
	}
}

// AddEvent logs a processor event
func (mp *MessageProcessor) AddEvent(ev ProcessorEvent) {
	mp.eventsMu.Lock()
	if len(mp.events) > 200 {
		mp.events = mp.events[1:]
	}
	mp.events = append(mp.events, ev)
	mp.eventsMu.Unlock()
	if ev.WorkerID >= 0 {
		if mp.workerHealth == nil {
			mp.workerHealth = make(map[int]*WorkerHealth)
		}
		h := mp.workerHealth[ev.WorkerID]
		if h == nil {
			h = &WorkerHealth{}
			mp.workerHealth[ev.WorkerID] = h
		}
		h.LastEvent = ev.Time
		if ev.Type == "panic" || ev.Type == "error" {
			h.LastError = ev.Error
			h.Healthy = false
		} else if ev.Type == "worker_start" {
			h.Healthy = true
		}
	}
}

// GetEvents returns recent processor events
func (mp *MessageProcessor) GetEvents() []ProcessorEvent {
	mp.eventsMu.Lock()
	defer mp.eventsMu.Unlock()
	return append([]ProcessorEvent(nil), mp.events...)
}

// GetWorkerHealth returns health info for all workers
func (mp *MessageProcessor) GetWorkerHealth() map[int]WorkerHealth {
	mp.eventsMu.Lock()
	defer mp.eventsMu.Unlock()
	result := make(map[int]WorkerHealth)
	for id, h := range mp.workerHealth {
		if h != nil {
			result[id] = *h
		}
	}
	return result
}

// SIMD-optimized CRC32 (placeholder - would use actual SIMD instructions)
func simdCRC32(data []byte) uint32 {
	// This would be implemented with actual SIMD instructions
	// For now, fall back to standard implementation
	return standardCRC32(data)
}

// Standard CRC32 implementation
func standardCRC32(data []byte) uint32 {
	var crc uint32 = 0xFFFFFFFF
	for _, b := range data {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 == 1 {
				crc = (crc >> 1) ^ 0xEDB88320
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}

// GetMetrics returns processor metrics
func (mp *MessageProcessor) GetMetrics() *ProcessorMetrics {
	return mp.metrics
}

// GetQueueLength returns current queue length
func (mp *MessageProcessor) GetQueueLength() int {
	return len(mp.processingQueue)
}

// GetBackpressureRatio returns current backpressure ratio
func (mp *MessageProcessor) GetBackpressureRatio() float64 {
	return float64(len(mp.processingQueue)) / float64(mp.config.QueueSize)
}

// Zero-copy utilities
func bytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func stringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}
