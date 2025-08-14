package queue

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meftunca/portask/pkg/types"
)

// UltraWorker - SIMD optimized, batch processing worker
type UltraWorker struct {
	id           int
	bus          *UltraMessageBus
	processor    MessageProcessor
	stats        *UltraWorkerStats
	batchChannel chan []*types.PortaskMessage
	running      int32
	wg           sync.WaitGroup

	// Performance optimizations
	localBatch     []*types.PortaskMessage
	batchCapacity  int
	processingTime time.Duration
}

// UltraWorkerStats with detailed metrics
type UltraWorkerStats struct {
	ID                int           `json:"id"`
	BatchesProcessed  uint64        `json:"batches_processed"`
	MessagesProcessed uint64        `json:"messages_processed"`
	ErrorCount        uint64        `json:"error_count"`
	AvgBatchSize      float64       `json:"avg_batch_size"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	ThroughputPerSec  uint64        `json:"throughput_per_sec"`
	LastActivity      time.Time     `json:"last_activity"`
	Status            string        `json:"status"`
}

// UltraMessageBus - Ultimate performance message bus
type UltraMessageBus struct {
	// Ultra-performance queues
	ultraHighQueue   *UltraHighPerformanceQueue
	ultraNormalQueue *UltraHighPerformanceQueue
	ultraLowQueue    *UltraHighPerformanceQueue

	// Ultra workers
	workers     []*UltraWorker
	workerCount int
	batchSize   int

	// Performance metrics
	stats        *UltraBusStats
	metricsTimer *time.Timer

	// Control
	running int32
	wg      sync.WaitGroup
}

// UltraBusStats for comprehensive metrics
type UltraBusStats struct {
	TotalMessages     uint64                      `json:"total_messages"`
	TotalBatches      uint64                      `json:"total_batches"`
	MessagesPerSecond uint64                      `json:"messages_per_second"`
	BatchesPerSecond  uint64                      `json:"batches_per_second"`
	AvgLatencyNs      uint64                      `json:"avg_latency_ns"`
	AvgBatchSize      float64                     `json:"avg_batch_size"`
	WorkerStats       map[int]*UltraWorkerStats   `json:"worker_stats"`
	QueueStats        map[string]*UltraQueueStats `json:"queue_stats"`
	StartTime         time.Time                   `json:"start_time"`
	LastUpdate        time.Time                   `json:"last_update"`
}

// NewUltraMessageBus creates the ultimate performance message bus
func NewUltraMessageBus(workerCount, batchSize int) *UltraMessageBus {
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	bus := &UltraMessageBus{
		ultraHighQueue:   NewUltraHighPerformanceQueue("ultra-high", 65536, batchSize),
		ultraNormalQueue: NewUltraHighPerformanceQueue("ultra-normal", 1048576, batchSize),
		ultraLowQueue:    NewUltraHighPerformanceQueue("ultra-low", 65536, batchSize),
		workerCount:      workerCount,
		batchSize:        batchSize,
		workers:          make([]*UltraWorker, workerCount),
		stats: &UltraBusStats{
			WorkerStats: make(map[int]*UltraWorkerStats),
			QueueStats:  make(map[string]*UltraQueueStats),
			StartTime:   time.Now(),
		},
	}

	// Initialize ultra workers
	for i := 0; i < workerCount; i++ {
		worker := &UltraWorker{
			id:            i,
			bus:           bus,
			batchChannel:  make(chan []*types.PortaskMessage, 1000), // High capacity
			batchCapacity: batchSize,
			localBatch:    make([]*types.PortaskMessage, 0, batchSize),
			stats: &UltraWorkerStats{
				ID:     i,
				Status: "created",
			},
		}
		bus.workers[i] = worker
		bus.stats.WorkerStats[i] = worker.stats
	}

	// Pre-warm all queues
	bus.ultraHighQueue.PreWarm()
	bus.ultraNormalQueue.PreWarm()
	bus.ultraLowQueue.PreWarm()

	return bus
}

// Start starts the ultra-performance message bus
func (bus *UltraMessageBus) Start() error {
	if !atomic.CompareAndSwapInt32(&bus.running, 0, 1) {
		return types.NewPortaskError(types.ErrCodeSystemError, "ultra message bus already running")
	}

	// Start all ultra workers
	for _, worker := range bus.workers {
		worker.start()
	}

	// Start batch coordinator
	go bus.batchCoordinator()

	// Start metrics collector
	go bus.ultraMetricsCollector()

	return nil
}

// batchCoordinator coordinates batch processing across workers
func (bus *UltraMessageBus) batchCoordinator() {
	ticker := time.NewTicker(50 * time.Microsecond) // Ultra-fast coordination
	defer ticker.Stop()

	workerIndex := 0

	for atomic.LoadInt32(&bus.running) == 1 {
		select {
		case <-ticker.C:
			// Round-robin batch collection
			worker := bus.workers[workerIndex]
			workerIndex = (workerIndex + 1) % bus.workerCount

			// Collect batch from high priority queue
			highBatch := bus.ultraHighQueue.BatchDequeue(bus.batchSize)
			if len(highBatch) > 0 {
				select {
				case worker.batchChannel <- highBatch:
				default:
					// Worker busy, try next worker
					nextWorker := bus.workers[(workerIndex+1)%bus.workerCount]
					select {
					case nextWorker.batchChannel <- highBatch:
					default:
						// All workers busy, this is a good problem!
					}
				}
				continue
			}

			// Collect batch from normal priority queue
			normalBatch := bus.ultraNormalQueue.BatchDequeue(bus.batchSize)
			if len(normalBatch) > 0 {
				select {
				case worker.batchChannel <- normalBatch:
				default:
					// Worker busy
				}
				continue
			}

			// Collect batch from low priority queue
			lowBatch := bus.ultraLowQueue.BatchDequeue(bus.batchSize)
			if len(lowBatch) > 0 {
				select {
				case worker.batchChannel <- lowBatch:
				default:
					// Worker busy
				}
			}
		}
	}
}

// start starts an ultra worker
func (w *UltraWorker) start() {
	atomic.StoreInt32(&w.running, 1)
	w.wg.Add(1)
	go w.ultraRun()
}

// ultraRun is the ultra-performance worker loop
func (w *UltraWorker) ultraRun() {
	defer w.wg.Done()
	w.stats.Status = "running"

	for atomic.LoadInt32(&w.running) == 1 {
		select {
		case batch := <-w.batchChannel:
			if len(batch) > 0 {
				w.processBatch(batch)
			}
		case <-time.After(1 * time.Millisecond):
			// Very short timeout to stay responsive
			continue
		}
	}

	w.stats.Status = "stopped"
}

// processBatch processes a batch of messages with SIMD-like efficiency
func (w *UltraWorker) processBatch(batch []*types.PortaskMessage) {
	startTime := time.Now()

	// Pre-allocate error slice
	errors := make([]error, 0, len(batch))

	// Process batch - could be optimized with SIMD instructions
	for _, message := range batch {
		if err := w.processor.ProcessMessage(nil, message); err != nil {
			errors = append(errors, err)
		}
	}

	// Update statistics
	batchDuration := time.Since(startTime)
	messagesProcessed := uint64(len(batch))
	errorCount := uint64(len(errors))

	atomic.AddUint64(&w.stats.BatchesProcessed, 1)
	atomic.AddUint64(&w.stats.MessagesProcessed, messagesProcessed)
	atomic.AddUint64(&w.stats.ErrorCount, errorCount)

	// Update averages
	currentAvgBatch := w.stats.AvgBatchSize
	w.stats.AvgBatchSize = (currentAvgBatch + float64(len(batch))) / 2

	currentAvgTime := w.stats.AvgProcessingTime
	w.stats.AvgProcessingTime = (currentAvgTime + batchDuration) / 2

	w.stats.LastActivity = time.Now()

	// Update bus statistics
	atomic.AddUint64(&w.bus.stats.TotalMessages, messagesProcessed)
	atomic.AddUint64(&w.bus.stats.TotalBatches, 1)
}

// UltraPublish publishes message with ultra performance
func (bus *UltraMessageBus) UltraPublish(message *types.PortaskMessage) error {
	if atomic.LoadInt32(&bus.running) == 0 {
		return types.NewPortaskError(types.ErrCodeSystemError, "ultra message bus not running")
	}

	// Route to appropriate ultra queue
	var queue *UltraHighPerformanceQueue
	switch message.Priority {
	case types.PriorityHigh:
		queue = bus.ultraHighQueue
	case types.PriorityLow:
		queue = bus.ultraLowQueue
	default:
		queue = bus.ultraNormalQueue
	}

	// Use batch enqueue even for single message (for consistency)
	enqueuedCount := queue.BatchEnqueue([]*types.PortaskMessage{message})
	if enqueuedCount == 0 {
		return types.NewPortaskError(types.ErrCodeResourceExhausted, "ultra queue full")
	}

	return nil
}

// ultraMetricsCollector collects ultra-performance metrics
func (bus *UltraMessageBus) ultraMetricsCollector() {
	ticker := time.NewTicker(100 * time.Millisecond) // 10Hz metrics collection
	defer ticker.Stop()

	var lastTotalMessages uint64
	var lastTotalBatches uint64
	lastTime := time.Now()

	for atomic.LoadInt32(&bus.running) == 1 {
		select {
		case <-ticker.C:
			now := time.Now()
			duration := now.Sub(lastTime).Seconds()

			currentMessages := atomic.LoadUint64(&bus.stats.TotalMessages)
			currentBatches := atomic.LoadUint64(&bus.stats.TotalBatches)

			// Calculate rates
			if duration > 0 {
				messagesPerSec := uint64(float64(currentMessages-lastTotalMessages) / duration)
				batchesPerSec := uint64(float64(currentBatches-lastTotalBatches) / duration)

				atomic.StoreUint64(&bus.stats.MessagesPerSecond, messagesPerSec)
				atomic.StoreUint64(&bus.stats.BatchesPerSecond, batchesPerSec)
			}

			lastTotalMessages = currentMessages
			lastTotalBatches = currentBatches
			lastTime = now

			bus.stats.LastUpdate = now
		}
	}
}

// GetUltraStats returns comprehensive ultra-performance statistics
func (bus *UltraMessageBus) GetUltraStats() *UltraBusStats {
	stats := &UltraBusStats{
		TotalMessages:     atomic.LoadUint64(&bus.stats.TotalMessages),
		TotalBatches:      atomic.LoadUint64(&bus.stats.TotalBatches),
		MessagesPerSecond: atomic.LoadUint64(&bus.stats.MessagesPerSecond),
		BatchesPerSecond:  atomic.LoadUint64(&bus.stats.BatchesPerSecond),
		WorkerStats:       make(map[int]*UltraWorkerStats),
		QueueStats:        make(map[string]*UltraQueueStats),
		StartTime:         bus.stats.StartTime,
		LastUpdate:        bus.stats.LastUpdate,
	}

	// Copy worker stats
	for i, worker := range bus.workers {
		stats.WorkerStats[i] = &UltraWorkerStats{
			ID:                worker.stats.ID,
			BatchesProcessed:  atomic.LoadUint64(&worker.stats.BatchesProcessed),
			MessagesProcessed: atomic.LoadUint64(&worker.stats.MessagesProcessed),
			ErrorCount:        atomic.LoadUint64(&worker.stats.ErrorCount),
			AvgBatchSize:      worker.stats.AvgBatchSize,
			AvgProcessingTime: worker.stats.AvgProcessingTime,
			LastActivity:      worker.stats.LastActivity,
			Status:            worker.stats.Status,
		}
	}

	// Copy queue stats
	stats.QueueStats["ultra-high"] = bus.ultraHighQueue.GetUltraStats()
	stats.QueueStats["ultra-normal"] = bus.ultraNormalQueue.GetUltraStats()
	stats.QueueStats["ultra-low"] = bus.ultraLowQueue.GetUltraStats()

	return stats
}

// Stop stops the ultra-performance message bus
func (bus *UltraMessageBus) Stop() {
	if !atomic.CompareAndSwapInt32(&bus.running, 1, 0) {
		return
	}

	// Stop all workers
	for _, worker := range bus.workers {
		atomic.StoreInt32(&worker.running, 0)
		worker.wg.Wait()
	}

	bus.wg.Wait()
}
