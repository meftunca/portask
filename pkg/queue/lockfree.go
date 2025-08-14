package queue

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/meftunca/portask/pkg/types"
)

// LockFreeQueue implements a high-performance lock-free MPMC queue
type LockFreeQueue struct {
	// Ring buffer parameters
	mask   uint64
	buffer []queueNode

	// Atomic counters
	enqueuePos uint64
	dequeuePos uint64

	// Statistics
	enqueueCount uint64
	dequeueCount uint64
	dropCount    uint64

	// Configuration
	name       string
	capacity   int
	dropPolicy DropPolicy
}

// queueNode represents a node in the lock-free queue
type queueNode struct {
	sequence uint64
	data     unsafe.Pointer
}

// DropPolicy defines what to do when queue is full
type DropPolicy int

const (
	DropOldest DropPolicy = iota // Drop oldest messages
	DropNewest                   // Drop newest messages
	Block                        // Block until space available
)

// NewLockFreeQueue creates a new lock-free queue
func NewLockFreeQueue(name string, capacity int, dropPolicy DropPolicy) *LockFreeQueue {
	// Ensure capacity is power of 2 for efficient masking
	if capacity&(capacity-1) != 0 {
		// Round up to next power of 2
		capacity = int(nextPowerOf2(uint64(capacity)))
	}

	q := &LockFreeQueue{
		name:       name,
		capacity:   capacity,
		mask:       uint64(capacity - 1),
		buffer:     make([]queueNode, capacity),
		dropPolicy: dropPolicy,
	}

	// Initialize sequence numbers
	for i := range q.buffer {
		q.buffer[i].sequence = uint64(i)
	}

	return q
}

// Enqueue adds an item to the queue
func (q *LockFreeQueue) Enqueue(item *types.PortaskMessage) bool {
	var node *queueNode
	var pos uint64
	retryCount := 0
	maxRetries := 1000 // Limit retries to avoid infinite loops

	for retryCount < maxRetries {
		pos = atomic.LoadUint64(&q.enqueuePos)
		node = &q.buffer[pos&q.mask]
		seq := atomic.LoadUint64(&node.sequence)
		var dif int64 = int64(seq) - int64(pos)

		if dif == 0 {
			// Slot is available
			if atomic.CompareAndSwapUint64(&q.enqueuePos, pos, pos+1) {
				break
			}
		} else if dif < 0 {
			// Queue is full
			switch q.dropPolicy {
			case DropNewest:
				atomic.AddUint64(&q.dropCount, 1)
				return false
			case DropOldest:
				// Try to advance dequeue position to make space
				atomic.CompareAndSwapUint64(&q.dequeuePos,
					atomic.LoadUint64(&q.dequeuePos), pos-uint64(q.capacity)+1)
				retryCount++
				continue
			case Block:
				// Wait a bit and retry
				retryCount++
				if retryCount%10 == 0 { // Was 100 - now 10 for faster response
					time.Sleep(1 * time.Millisecond) // Was 10 microseconds - now 1ms
				} else {
					time.Sleep(100 * time.Microsecond) // Was runtime.Gosched() - now small sleep
				}
				continue
			}
		} else {
			// Another thread is working on this slot
			retryCount++
			if retryCount%10 == 0 { // Was 100 - now 10
				time.Sleep(100 * time.Microsecond) // Was runtime.Gosched() - now small sleep
			}
		}
	}

	if retryCount >= maxRetries {
		// Avoid infinite loop, drop message
		atomic.AddUint64(&q.dropCount, 1)
		return false
	}

	// Store the item
	atomic.StorePointer(&node.data, unsafe.Pointer(item))
	atomic.StoreUint64(&node.sequence, pos+1)
	atomic.AddUint64(&q.enqueueCount, 1)

	return true
}

// Dequeue removes an item from the queue
func (q *LockFreeQueue) Dequeue() (*types.PortaskMessage, bool) {
	var node *queueNode
	var pos uint64
	retryCount := 0
	maxRetries := 1000 // Limit retries to avoid infinite loops

	for retryCount < maxRetries {
		pos = atomic.LoadUint64(&q.dequeuePos)
		node = &q.buffer[pos&q.mask]
		seq := atomic.LoadUint64(&node.sequence)
		var dif int64 = int64(seq) - int64(pos+1)

		if dif == 0 {
			// Item is available
			if atomic.CompareAndSwapUint64(&q.dequeuePos, pos, pos+1) {
				break
			}
		} else if dif < 0 {
			// Queue is empty
			return nil, false
		} else {
			// Another thread is working on this slot
			retryCount++
			if retryCount%100 == 0 {
				// Yield CPU every 100 retries
				runtime.Gosched()
			}
		}
	}

	if retryCount >= maxRetries {
		// Avoid infinite loop, return empty
		return nil, false
	}

	// Load the item
	data := atomic.LoadPointer(&node.data)
	atomic.StorePointer(&node.data, nil)
	atomic.StoreUint64(&node.sequence, pos+q.mask+1)
	atomic.AddUint64(&q.dequeueCount, 1)

	return (*types.PortaskMessage)(data), true
}

// TryDequeue attempts to dequeue without blocking
func (q *LockFreeQueue) TryDequeue() (*types.PortaskMessage, bool) {
	return q.Dequeue() // Already non-blocking
}

// Size returns approximate queue size
func (q *LockFreeQueue) Size() int {
	enq := atomic.LoadUint64(&q.enqueuePos)
	deq := atomic.LoadUint64(&q.dequeuePos)
	return int(enq - deq)
}

// IsEmpty returns true if queue appears empty
func (q *LockFreeQueue) IsEmpty() bool {
	return q.Size() <= 0
}

// IsFull returns true if queue appears full
func (q *LockFreeQueue) IsFull() bool {
	return q.Size() >= q.capacity
}

// Stats returns queue statistics
func (q *LockFreeQueue) Stats() QueueStats {
	return QueueStats{
		Name:         q.name,
		Capacity:     q.capacity,
		Size:         q.Size(),
		EnqueueCount: atomic.LoadUint64(&q.enqueueCount),
		DequeueCount: atomic.LoadUint64(&q.dequeueCount),
		DropCount:    atomic.LoadUint64(&q.dropCount),
	}
}

// tryForceEnqueueDropOldest drops one oldest item (if any) and retries enqueue once.
// Only active when drop policy is DropOldest. Returns true if enqueue succeeded.
func (q *LockFreeQueue) tryForceEnqueueDropOldest(item *types.PortaskMessage) bool {
	if q.dropPolicy != DropOldest {
		return false
	}
	// Drop one item from the head to make room.
	_, _ = q.Dequeue()
	return q.Enqueue(item)
}

// QueueStats represents queue statistics
type QueueStats struct {
	Name         string `json:"name"`
	Capacity     int    `json:"capacity"`
	Size         int    `json:"size"`
	EnqueueCount uint64 `json:"enqueue_count"`
	DequeueCount uint64 `json:"dequeue_count"`
	DropCount    uint64 `json:"drop_count"`
}

// MessageBus coordinates message routing and processing
type MessageBus struct {
	// Queues for different priority levels
	highPriorityQueue   *LockFreeQueue
	normalPriorityQueue *LockFreeQueue
	lowPriorityQueue    *LockFreeQueue

	// Worker management
	workers    []*Worker
	workerPool *WorkerPool

	// Topic routing
	topicQueues map[types.TopicName]*LockFreeQueue
	topicMutex  sync.RWMutex

	// Statistics
	stats *BusStats

	// Control
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running int32

	// Worker notification for zero-CPU idle
	workerNotify chan struct{}
}

// BusStats represents message bus statistics
type BusStats struct {
	TotalMessages     uint64                `json:"total_messages"`
	MessagesPerSecond float64               `json:"messages_per_second"`
	TotalBytes        uint64                `json:"total_bytes"`
	BytesPerSecond    float64               `json:"bytes_per_second"`
	QueueStats        map[string]QueueStats `json:"queue_stats"`
	WorkerStats       []WorkerStats         `json:"worker_stats"`
	TopicStats        map[string]uint64     `json:"topic_stats"`
	LastUpdate        time.Time             `json:"last_update"`
}

// NewMessageBus creates a new message bus
func NewMessageBus(config MessageBusConfig) *MessageBus {
	ctx, cancel := context.WithCancel(context.Background())

	mb := &MessageBus{
		highPriorityQueue:   NewLockFreeQueue("high-priority", config.HighPriorityQueueSize, config.DropPolicy),
		normalPriorityQueue: NewLockFreeQueue("normal-priority", config.NormalPriorityQueueSize, config.DropPolicy),
		lowPriorityQueue:    NewLockFreeQueue("low-priority", config.LowPriorityQueueSize, config.DropPolicy),
		topicQueues:         make(map[types.TopicName]*LockFreeQueue),
		ctx:                 ctx,
		cancel:              cancel,
		workerNotify:        make(chan struct{}, 1000), // Buffered notification channel
		stats: &BusStats{
			QueueStats: make(map[string]QueueStats),
			TopicStats: make(map[string]uint64),
			LastUpdate: time.Now(),
		},
	}

	// Initialize worker pool
	mb.workerPool = NewWorkerPool(config.WorkerPoolConfig, mb)

	return mb
}

// MessageBusConfig configures the message bus
type MessageBusConfig struct {
	HighPriorityQueueSize   int
	NormalPriorityQueueSize int
	LowPriorityQueueSize    int
	DropPolicy              DropPolicy
	WorkerPoolConfig        WorkerPoolConfig
	EnableTopicQueues       bool
}

// Start starts the message bus
func (mb *MessageBus) Start() error {
	if !atomic.CompareAndSwapInt32(&mb.running, 0, 1) {
		return types.NewPortaskError(types.ErrCodeSystemError, "message bus already running")
	}

	// Start worker pool
	if err := mb.workerPool.Start(mb.ctx); err != nil {
		return err
	}

	// Start statistics collector
	mb.wg.Add(1)
	go mb.statsCollector()

	return nil
}

// Stop stops the message bus
func (mb *MessageBus) Stop() error {
	if !atomic.CompareAndSwapInt32(&mb.running, 1, 0) {
		return nil // Already stopped
	}

	mb.cancel()
	mb.wg.Wait()

	return mb.workerPool.Stop()
}

// Publish publishes a message to the bus
func (mb *MessageBus) Publish(message *types.PortaskMessage) error {
	if atomic.LoadInt32(&mb.running) == 0 {
		return types.NewPortaskError(types.ErrCodeSystemError, "message bus not running")
	}

	// Update statistics
	atomic.AddUint64(&mb.stats.TotalMessages, 1)
	atomic.AddUint64(&mb.stats.TotalBytes, uint64(message.GetSize()))

	// Route to appropriate queue based on priority
	var queue *LockFreeQueue
	switch message.Priority {
	case types.PriorityHigh:
		queue = mb.highPriorityQueue
	case types.PriorityLow:
		queue = mb.lowPriorityQueue
	default:
		queue = mb.normalPriorityQueue
	}

	// Try to enqueue
	if !queue.Enqueue(message) {
		// If policy is DropOldest, aggressively make space and avoid surfacing an error.
		if queue.dropPolicy == DropOldest {
			if queue.tryForceEnqueueDropOldest(message) {
				return nil
			}
			// As a last resort, count a drop and accept without enqueuing to preserve publisher flow.
			atomic.AddUint64(&queue.dropCount, 1)
			return nil
		}
		return types.NewPortaskError(types.ErrCodeResourceExhausted, "queue full").
			WithDetail("queue", queue.name).
			WithDetail("priority", message.Priority)
	}

	// Notify workers that new message is available
	mb.notifyWorkers()

	return nil
}

// notifyWorkers wakes up ALL workers for maximum processing power
func (mb *MessageBus) notifyWorkers() {
	// AGGRESSIVE: Notify ALL workers for ultra performance
	for _, worker := range mb.workers {
		select {
		case worker.notify <- struct{}{}:
			// Worker notified successfully
		default:
			// Worker channel full, already notified
		}
	}

	// Also notify via bus channel for backup
	select {
	case mb.workerNotify <- struct{}{}:
	default:
		// Bus notification channel full
	}
}

// PublishToPriority publishes a message to a specific priority queue
func (mb *MessageBus) PublishToPriority(message *types.PortaskMessage, priority types.MessagePriority) error {
	if atomic.LoadInt32(&mb.running) == 0 {
		return types.NewPortaskError(types.ErrCodeSystemError, "message bus not running")
	}

	// Override message priority
	message.Priority = priority

	// Update statistics
	atomic.AddUint64(&mb.stats.TotalMessages, 1)
	atomic.AddUint64(&mb.stats.TotalBytes, uint64(message.GetSize()))

	// Route to appropriate queue based on priority
	var queue *LockFreeQueue
	switch priority {
	case types.PriorityHigh:
		queue = mb.highPriorityQueue
	case types.PriorityLow:
		queue = mb.lowPriorityQueue
	default:
		queue = mb.normalPriorityQueue
	}

	// Try to enqueue
	if !queue.Enqueue(message) {
		// If policy is DropOldest, aggressively make space and avoid surfacing an error.
		if queue.dropPolicy == DropOldest {
			if queue.tryForceEnqueueDropOldest(message) {
				return nil
			}
			// As a last resort, count a drop and accept without enqueuing to preserve publisher flow.
			atomic.AddUint64(&queue.dropCount, 1)
			return nil
		}
		return types.NewPortaskError(types.ErrCodeResourceExhausted, "queue full").
			WithDetail("queue", queue.name).
			WithDetail("priority", priority)
	}

	return nil
}

// PublishToTopic publishes a message to a specific topic queue
func (mb *MessageBus) PublishToTopic(message *types.PortaskMessage) error {
	if atomic.LoadInt32(&mb.running) == 0 {
		return types.NewPortaskError(types.ErrCodeSystemError, "message bus not running")
	}

	// Get or create topic queue
	queue := mb.getOrCreateTopicQueue(message.Topic)

	// Update statistics
	atomic.AddUint64(&mb.stats.TotalMessages, 1)
	atomic.AddUint64(&mb.stats.TotalBytes, uint64(message.GetSize()))

	mb.topicMutex.RLock()
	if count, exists := mb.stats.TopicStats[string(message.Topic)]; exists {
		mb.stats.TopicStats[string(message.Topic)] = count + 1
	} else {
		mb.stats.TopicStats[string(message.Topic)] = 1
	}
	mb.topicMutex.RUnlock()

	// Try to enqueue
	if !queue.Enqueue(message) {
		return types.NewPortaskError(types.ErrCodeResourceExhausted, "topic queue full").
			WithDetail("topic", message.Topic)
	}

	return nil
}

// getOrCreateTopicQueue gets or creates a queue for a topic
func (mb *MessageBus) getOrCreateTopicQueue(topic types.TopicName) *LockFreeQueue {
	mb.topicMutex.RLock()
	if queue, exists := mb.topicQueues[topic]; exists {
		mb.topicMutex.RUnlock()
		return queue
	}
	mb.topicMutex.RUnlock()

	mb.topicMutex.Lock()
	defer mb.topicMutex.Unlock()

	// Double-check after acquiring write lock
	if queue, exists := mb.topicQueues[topic]; exists {
		return queue
	}

	// Create new topic queue
	queue := NewLockFreeQueue(string(topic), 1000, DropOldest) // Default config
	mb.topicQueues[topic] = queue

	return queue
}

// GetStats returns current bus statistics
func (mb *MessageBus) GetStats() *BusStats {
	mb.topicMutex.RLock()
	defer mb.topicMutex.RUnlock()

	stats := &BusStats{
		TotalMessages: atomic.LoadUint64(&mb.stats.TotalMessages),
		TotalBytes:    atomic.LoadUint64(&mb.stats.TotalBytes),
		QueueStats:    make(map[string]QueueStats),
		WorkerStats:   mb.workerPool.GetStats(),
		TopicStats:    make(map[string]uint64),
		LastUpdate:    time.Now(),
	}

	// Copy queue stats
	stats.QueueStats["high-priority"] = mb.highPriorityQueue.Stats()
	stats.QueueStats["normal-priority"] = mb.normalPriorityQueue.Stats()
	stats.QueueStats["low-priority"] = mb.lowPriorityQueue.Stats()

	// Copy topic stats
	for topic, count := range mb.stats.TopicStats {
		stats.TopicStats[topic] = count
	}

	// Copy topic queue stats
	for topic, queue := range mb.topicQueues {
		stats.QueueStats[string(topic)] = queue.Stats()
	}

	return stats
}

// statsCollector collects statistics periodically
func (mb *MessageBus) statsCollector() {
	defer mb.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastMessages, lastBytes uint64
	var lastTime time.Time = time.Now()

	for {
		select {
		case <-mb.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			duration := now.Sub(lastTime)

			currentMessages := atomic.LoadUint64(&mb.stats.TotalMessages)
			currentBytes := atomic.LoadUint64(&mb.stats.TotalBytes)

			if duration.Seconds() > 0 {
				mb.stats.MessagesPerSecond = float64(currentMessages-lastMessages) / duration.Seconds()
				mb.stats.BytesPerSecond = float64(currentBytes-lastBytes) / duration.Seconds()
			}

			lastMessages = currentMessages
			lastBytes = currentBytes
			lastTime = now
			mb.stats.LastUpdate = now
		}
	}
}

// Worker represents a message processing worker
type Worker struct {
	id        int
	bus       *MessageBus
	processor MessageProcessor
	stats     *WorkerStats
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   int32
	// Event-driven notification channel
	notify chan struct{}
}

// WorkerStats represents worker statistics
type WorkerStats struct {
	ID                int           `json:"id"`
	MessagesProcessed uint64        `json:"messages_processed"`
	ErrorCount        uint64        `json:"error_count"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	LastActivity      time.Time     `json:"last_activity"`
	Status            string        `json:"status"`
}

// MessageProcessor processes messages
type MessageProcessor interface {
	ProcessMessage(ctx context.Context, message *types.PortaskMessage) error
}

// WorkerPool manages a pool of workers
type WorkerPool struct {
	workers []*Worker
	config  WorkerPoolConfig
	bus     *MessageBus
	running int32
}

// WorkerPoolConfig configures the worker pool
type WorkerPoolConfig struct {
	WorkerCount      int
	MessageProcessor MessageProcessor
	BatchSize        int
	BatchTimeout     time.Duration
	EnableProfiling  bool
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(config WorkerPoolConfig, bus *MessageBus) *WorkerPool {
	wp := &WorkerPool{
		config:  config,
		bus:     bus,
		workers: make([]*Worker, config.WorkerCount),
	}

	// Create workers
	for i := 0; i < config.WorkerCount; i++ {
		wp.workers[i] = &Worker{
			id:        i,
			bus:       bus,
			processor: config.MessageProcessor,
			notify:    make(chan struct{}, 1), // Individual worker notification channel
			stats: &WorkerStats{
				ID:     i,
				Status: "created",
			},
		}
	}

	return wp
}

// Start starts all workers
func (wp *WorkerPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&wp.running, 0, 1) {
		return types.NewPortaskError(types.ErrCodeSystemError, "worker pool already running")
	}

	for _, worker := range wp.workers {
		if err := worker.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Stop stops all workers
func (wp *WorkerPool) Stop() error {
	if !atomic.CompareAndSwapInt32(&wp.running, 1, 0) {
		return nil
	}

	for _, worker := range wp.workers {
		worker.Stop()
	}

	return nil
}

// GetStats returns worker pool statistics
func (wp *WorkerPool) GetStats() []WorkerStats {
	stats := make([]WorkerStats, len(wp.workers))
	for i, worker := range wp.workers {
		stats[i] = *worker.stats
	}
	return stats
}

// Start starts the worker
func (w *Worker) Start(parentCtx context.Context) error {
	if !atomic.CompareAndSwapInt32(&w.running, 0, 1) {
		return types.NewPortaskError(types.ErrCodeSystemError, "worker already running")
	}

	w.ctx, w.cancel = context.WithCancel(parentCtx)
	w.stats.Status = "running"

	w.wg.Add(1)
	go w.run()

	return nil
}

// Stop stops the worker
func (w *Worker) Stop() {
	if atomic.CompareAndSwapInt32(&w.running, 1, 0) {
		w.cancel()
		w.wg.Wait()
		w.stats.Status = "stopped"
	}
}

// run is the MEGA ULTRA worker loop - maximum processing power
func (w *Worker) run() {
	defer w.wg.Done()

	w.stats.Status = "running"

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.notify:
			// Worker woke up - MEGA AGGRESSIVE processing
			w.megaProcessAllAvailableMessages()
		case <-time.After(10 * time.Millisecond): // Faster periodic check (was 1s)
			// More frequent check for ultra performance
			w.megaProcessAllAvailableMessages()
		default:
			// ULTRA AGGRESSIVE: Always try to process when not blocked
			if w.megaProcessAllAvailableMessages() {
				// If we processed something, immediately try again
				continue
			}
			// Only yield if nothing to process
			time.Sleep(100 * time.Microsecond) // Tiny sleep
		}
	}
}

// megaProcessAllAvailableMessages - ULTRA AGGRESSIVE batch processing
func (w *Worker) megaProcessAllAvailableMessages() bool {
	processedAny := false
	batchCount := 0
	maxBatches := 100 // Process up to 100 batches in one go

	for batchCount < maxBatches {
		batchProcessed := false
		messagesInBatch := 0

		// Try to process MEGA batches from each queue
		for i := 0; i < 50; i++ { // Up to 50 messages per queue per batch
			if w.processFromQueue(w.bus.highPriorityQueue) {
				batchProcessed = true
				messagesInBatch++
				processedAny = true
			} else {
				break
			}
		}

		// Process normal priority in larger batches
		for i := 0; i < 100; i++ { // Up to 100 messages per batch
			if w.processFromQueue(w.bus.normalPriorityQueue) {
				batchProcessed = true
				messagesInBatch++
				processedAny = true
			} else {
				break
			}
		}

		// Process low priority
		for i := 0; i < 25; i++ { // Up to 25 messages per batch
			if w.processFromQueue(w.bus.lowPriorityQueue) {
				batchProcessed = true
				messagesInBatch++
				processedAny = true
			} else {
				break
			}
		}

		// Process topic queues
		w.bus.topicMutex.RLock()
		for _, queue := range w.bus.topicQueues {
			for i := 0; i < 50; i++ { // Up to 50 per topic queue
				if w.processFromQueue(queue) {
					batchProcessed = true
					messagesInBatch++
					processedAny = true
				} else {
					break
				}
			}
		}
		w.bus.topicMutex.RUnlock()

		if !batchProcessed {
			break // No more messages available
		}

		batchCount++

		// If we processed a lot, yield briefly to other goroutines
		if messagesInBatch > 200 {
			runtime.Gosched()
		}
	}

	return processedAny
}

// processFromQueue processes a message from the specified queue
func (w *Worker) processFromQueue(queue *LockFreeQueue) bool {
	message, ok := queue.TryDequeue()
	if !ok {
		return false
	}

	start := time.Now()
	err := w.processor.ProcessMessage(w.ctx, message)
	duration := time.Since(start)

	// Update statistics
	atomic.AddUint64(&w.stats.MessagesProcessed, 1)
	if err != nil {
		atomic.AddUint64(&w.stats.ErrorCount, 1)
	}

	// Update average processing time (simple moving average)
	if w.stats.AvgProcessingTime == 0 {
		w.stats.AvgProcessingTime = duration
	} else {
		w.stats.AvgProcessingTime = (w.stats.AvgProcessingTime + duration) / 2
	}

	w.stats.LastActivity = time.Now()

	return true
}

// nextPowerOf2 returns the next power of 2 >= n
func nextPowerOf2(n uint64) uint64 {
	if n == 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	n++
	return n
}
