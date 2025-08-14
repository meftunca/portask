package rq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/meftunca/portask/pkg/queue"
	"github.com/meftunca/portask/pkg/types"
)

// RQAdapter provides RQ compatibility for Portask
type RQAdapter struct {
	messageBus *queue.MessageBus
	functions  map[string]JobFunction
	workers    map[string]*Worker
	queues     map[string]*RQQueue
	scheduler  *JobScheduler
	stats      *RQStats
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	eventChan  chan JobEvent
}

// RQQueue represents an RQ-compatible queue
type RQQueue struct {
	Name     string
	Priority Priority
	Config   QueueConfig
	JobCount int64
	mu       sync.RWMutex
}

// NewRQAdapter creates a new RQ adapter
func NewRQAdapter(messageBus *queue.MessageBus) *RQAdapter {
	ctx, cancel := context.WithCancel(context.Background())

	adapter := &RQAdapter{
		messageBus: messageBus,
		functions:  make(map[string]JobFunction),
		workers:    make(map[string]*Worker),
		queues:     make(map[string]*RQQueue),
		scheduler:  NewJobScheduler(),
		stats: &RQStats{
			QueueStats: make(map[string]*JobStats),
			UpdatedAt:  time.Now(),
		},
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan JobEvent, 1000),
	}

	// Create default queues
	adapter.createDefaultQueues()

	// Start background processes
	go adapter.statsUpdater()
	go adapter.eventProcessor()

	return adapter
}

// createDefaultQueues creates the standard RQ queues
func (r *RQAdapter) createDefaultQueues() {
	defaultQueues := []QueueConfig{
		{Name: "high", Priority: PriorityHigh, MaxSize: 10000, Persistent: true},
		{Name: "default", Priority: PriorityNormal, MaxSize: 50000, Persistent: true},
		{Name: "low", Priority: PriorityLow, MaxSize: 100000, Persistent: true},
		{Name: "failed", Priority: PriorityNormal, MaxSize: 10000, Persistent: true},
		{Name: "scheduled", Priority: PriorityNormal, MaxSize: 50000, Persistent: true},
	}

	for _, config := range defaultQueues {
		r.CreateQueue(config)
	}
}

// Enqueue adds a job to the queue
func (r *RQAdapter) Enqueue(job *Job) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	job.Status = StatusQueued
	job.CreatedAt = time.Now()

	// Convert to Portask message
	message := &types.PortaskMessage{
		ID:        types.MessageID(job.ID),
		Topic:     types.TopicName(job.Queue),
		Timestamp: time.Now().Unix(),
		Payload:   r.serializeJob(job),
		Priority:  r.mapPriority(job.Priority),
		Headers: types.MessageHeaders{
			"job_type":    "rq",
			"function":    job.Function,
			"status":      string(job.Status),
			"retry_count": "0",
		},
	}

	// Publish to appropriate priority queue
	var err error
	switch job.Priority {
	case PriorityHigh:
		err = r.messageBus.PublishToPriority(message, types.PriorityHigh)
	case PriorityLow:
		err = r.messageBus.PublishToPriority(message, types.PriorityLow)
	default:
		err = r.messageBus.PublishToPriority(message, types.PriorityNormal)
	}

	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Update queue stats
	r.updateQueueStats(job.Queue, 1, 0, 0, 0)

	// Send event
	r.sendEvent(JobEvent{
		JobID:     job.ID,
		OldStatus: "",
		NewStatus: StatusQueued,
		Timestamp: time.Now(),
	})

	return nil
}

// EnqueueAt schedules a job to run at a specific time
func (r *RQAdapter) EnqueueAt(job *Job, at time.Time) error {
	job.ScheduledAt = &at
	job.Status = StatusScheduled

	return r.scheduler.Schedule(job, at)
}

// EnqueueIn schedules a job to run after a delay
func (r *RQAdapter) EnqueueIn(job *Job, delay time.Duration) error {
	at := time.Now().Add(delay)
	return r.EnqueueAt(job, at)
}

// GetJob retrieves a job by ID
func (r *RQAdapter) GetJob(jobID string) (*Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// This would typically query from storage
	// For now, we'll return a placeholder implementation
	return nil, fmt.Errorf("job not found: %s", jobID)
}

// CancelJob cancels a job
func (r *RQAdapter) CancelJob(jobID string) error {
	// Implementation for job cancellation
	r.sendEvent(JobEvent{
		JobID:     jobID,
		NewStatus: StatusCanceled,
		Timestamp: time.Now(),
	})

	return nil
}

// RequeueJob requeues a failed job
func (r *RQAdapter) RequeueJob(jobID string) error {
	// Implementation for job requeuing
	return nil
}

// CreateQueue creates a new queue
func (r *RQAdapter) CreateQueue(config QueueConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.queues[config.Name]; exists {
		return fmt.Errorf("queue already exists: %s", config.Name)
	}

	queue := &RQQueue{
		Name:     config.Name,
		Priority: config.Priority,
		Config:   config,
		JobCount: 0,
	}

	r.queues[config.Name] = queue
	r.stats.QueueStats[config.Name] = &JobStats{
		QueueName: config.Name,
		UpdatedAt: time.Now(),
	}
	r.stats.Queues++

	log.Printf("[RQ] Created queue: %s (priority: %s)", config.Name, config.Priority.String())
	return nil
}

// DeleteQueue deletes a queue
func (r *RQAdapter) DeleteQueue(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.queues[name]; !exists {
		return fmt.Errorf("queue not found: %s", name)
	}

	delete(r.queues, name)
	delete(r.stats.QueueStats, name)
	r.stats.Queues--

	log.Printf("[RQ] Deleted queue: %s", name)
	return nil
}

// PurgeQueue removes all jobs from a queue
func (r *RQAdapter) PurgeQueue(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	queue, exists := r.queues[name]
	if !exists {
		return fmt.Errorf("queue not found: %s", name)
	}

	queue.JobCount = 0
	log.Printf("[RQ] Purged queue: %s", name)
	return nil
}

// GetQueueLength returns the number of jobs in a queue
func (r *RQAdapter) GetQueueLength(name string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	queue, exists := r.queues[name]
	if !exists {
		return 0, fmt.Errorf("queue not found: %s", name)
	}

	return int(queue.JobCount), nil
}

// GetQueueJobs returns jobs from a queue
func (r *RQAdapter) GetQueueJobs(name string, offset, limit int) ([]*Job, error) {
	// Implementation for retrieving jobs from queue
	return []*Job{}, nil
}

// StartWorker starts a new worker
func (r *RQAdapter) StartWorker(config WorkerConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.workers[config.Name]; exists {
		return fmt.Errorf("worker already exists: %s", config.Name)
	}

	worker := NewWorker(config, r)
	r.workers[config.Name] = worker
	r.stats.Workers++

	go worker.Start(r.ctx)

	log.Printf("[RQ] Started worker: %s (queues: %v, concurrency: %d)",
		config.Name, config.Queues, config.Concurrency)
	return nil
}

// StopWorker stops a worker
func (r *RQAdapter) StopWorker(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	worker, exists := r.workers[name]
	if !exists {
		return fmt.Errorf("worker not found: %s", name)
	}

	worker.Stop()
	delete(r.workers, name)
	r.stats.Workers--

	log.Printf("[RQ] Stopped worker: %s", name)
	return nil
}

// GetWorkers returns information about all workers
func (r *RQAdapter) GetWorkers() ([]WorkerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workers := make([]WorkerInfo, 0, len(r.workers))
	for _, worker := range r.workers {
		workers = append(workers, worker.GetInfo())
	}

	return workers, nil
}

// GetStats returns RQ statistics
func (r *RQAdapter) GetStats() (*RQStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *r.stats
	stats.UpdatedAt = time.Now()

	return &stats, nil
}

// GetJobStats returns job statistics for a queue
func (r *RQAdapter) GetJobStats(queueName string) (*JobStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats, exists := r.stats.QueueStats[queueName]
	if !exists {
		return nil, fmt.Errorf("queue not found: %s", queueName)
	}

	// Create a copy
	statsCopy := *stats
	return &statsCopy, nil
}

// RegisterFunction registers a job function
func (r *RQAdapter) RegisterFunction(name string, fn JobFunction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.functions[name] = fn
	log.Printf("[RQ] Registered function: %s", name)
	return nil
}

// UnregisterFunction unregisters a job function
func (r *RQAdapter) UnregisterFunction(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.functions, name)
	log.Printf("[RQ] Unregistered function: %s", name)
	return nil
}

// Start starts the RQ adapter
func (r *RQAdapter) Start() error {
	log.Println("[RQ] Starting RQ adapter...")
	return nil
}

// Stop stops the RQ adapter
func (r *RQAdapter) Stop() error {
	log.Println("[RQ] Stopping RQ adapter...")

	r.cancel()

	// Stop all workers
	r.mu.Lock()
	for name, worker := range r.workers {
		worker.Stop()
		delete(r.workers, name)
	}
	r.mu.Unlock()

	return nil
}

// Health checks the health of the RQ adapter
func (r *RQAdapter) Health() error {
	return nil
}

// Helper methods

func (r *RQAdapter) serializeJob(job *Job) []byte {
	// Implementation for job serialization
	// This would use CBOR serialization
	return []byte{}
}

func (r *RQAdapter) mapPriority(priority Priority) types.MessagePriority {
	switch priority {
	case PriorityHigh:
		return types.PriorityHigh
	case PriorityLow:
		return types.PriorityLow
	default:
		return types.PriorityNormal
	}
}

func (r *RQAdapter) updateQueueStats(queueName string, queued, started, finished, failed int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats, exists := r.stats.QueueStats[queueName]
	if !exists {
		stats = &JobStats{QueueName: queueName}
		r.stats.QueueStats[queueName] = stats
	}

	stats.Queued += queued
	stats.Started += started
	stats.Finished += finished
	stats.Failed += failed
	stats.Total += queued + started + finished + failed
	stats.UpdatedAt = time.Now()

	// Update global stats
	r.stats.QueuedJobs += queued
	r.stats.StartedJobs += started
	r.stats.FinishedJobs += finished
	r.stats.FailedJobs += failed
	r.stats.TotalJobs += queued + started + finished + failed
}

func (r *RQAdapter) sendEvent(event JobEvent) {
	select {
	case r.eventChan <- event:
	default:
		// Channel full, drop event
	}
}

func (r *RQAdapter) statsUpdater() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			r.stats.UpdatedAt = time.Now()
			r.mu.Unlock()
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *RQAdapter) eventProcessor() {
	for {
		select {
		case event := <-r.eventChan:
			// Process job events (logging, metrics, webhooks, etc.)
			log.Printf("[RQ] Job event: %s -> %s (job: %s)",
				event.OldStatus, event.NewStatus, event.JobID)
		case <-r.ctx.Done():
			return
		}
	}
}
