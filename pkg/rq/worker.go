package rq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Worker represents an RQ worker that processes jobs
type Worker struct {
	config        WorkerConfig
	adapter       *RQAdapter
	status        WorkerStatus
	currentJob    *Job
	jobsProcessed int64
	startedAt     time.Time
	lastSeen      time.Time
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex
}

// WorkerStatus represents the current status of a worker
type WorkerStatus string

const (
	WorkerStatusIdle    WorkerStatus = "idle"
	WorkerStatusBusy    WorkerStatus = "busy"
	WorkerStatusStopped WorkerStatus = "stopped"
	WorkerStatusFailed  WorkerStatus = "failed"
)

// NewWorker creates a new worker
func NewWorker(config WorkerConfig, adapter *RQAdapter) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		config:        config,
		adapter:       adapter,
		status:        WorkerStatusIdle,
		startedAt:     time.Now(),
		lastSeen:      time.Now(),
		ctx:           ctx,
		cancel:        cancel,
		jobsProcessed: 0,
	}
}

// Start starts the worker
func (w *Worker) Start(parentCtx context.Context) {
	log.Printf("[Worker %s] Starting worker for queues: %v", w.config.Name, w.config.Queues)

	w.mu.Lock()
	w.status = WorkerStatusIdle
	w.startedAt = time.Now()
	w.mu.Unlock()

	// Start multiple goroutines for concurrency
	for i := 0; i < w.config.Concurrency; i++ {
		w.wg.Add(1)
		go w.workerLoop(i)
	}

	// Heartbeat goroutine
	w.wg.Add(1)
	go w.heartbeat()

	w.wg.Wait()
}

// Stop stops the worker gracefully
func (w *Worker) Stop() {
	log.Printf("[Worker %s] Stopping worker...", w.config.Name)

	w.mu.Lock()
	w.status = WorkerStatusStopped
	w.mu.Unlock()

	w.cancel()

	// Wait for graceful shutdown
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[Worker %s] Worker stopped gracefully", w.config.Name)
	case <-time.After(w.config.GracefulTimeout):
		log.Printf("[Worker %s] Worker force stopped after timeout", w.config.Name)
	}
}

// GetInfo returns worker information
func (w *Worker) GetInfo() WorkerInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return WorkerInfo{
		Name:          w.config.Name,
		Queues:        w.config.Queues,
		Status:        string(w.status),
		CurrentJob:    w.currentJob,
		JobsProcessed: atomic.LoadInt64(&w.jobsProcessed),
		StartedAt:     w.startedAt,
		LastSeen:      w.lastSeen,
	}
}

// workerLoop is the main worker processing loop
func (w *Worker) workerLoop(workerID int) {
	defer w.wg.Done()

	log.Printf("[Worker %s-%d] Starting worker loop", w.config.Name, workerID)

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[Worker %s-%d] Worker loop stopped", w.config.Name, workerID)
			return
		default:
			// Check if we've exceeded max jobs
			if w.config.MaxJobs > 0 && atomic.LoadInt64(&w.jobsProcessed) >= int64(w.config.MaxJobs) {
				log.Printf("[Worker %s-%d] Reached max jobs limit (%d), stopping",
					w.config.Name, workerID, w.config.MaxJobs)
				w.cancel()
				return
			}

			// Try to get a job from queues (priority order)
			job := w.getNextJob()
			if job == nil {
				// No jobs available, wait before polling again
				select {
				case <-time.After(w.config.PollInterval):
					continue
				case <-w.ctx.Done():
					return
				}
			}

			// Process the job
			w.processJob(job, workerID)
		}
	}
}

// getNextJob retrieves the next job from configured queues
func (w *Worker) getNextJob() *Job {
	// Try each queue in priority order
	for _, queueName := range w.config.Queues {
		// This would typically fetch from the message bus
		// For now, we'll return nil (no jobs available)
		// In a real implementation, this would:
		// 1. Check the queue for messages
		// 2. Deserialize the job
		// 3. Return the job
		_ = queueName
	}

	return nil
}

// processJob processes a single job
func (w *Worker) processJob(job *Job, workerID int) {
	log.Printf("[Worker %s-%d] Processing job: %s (%s)",
		w.config.Name, workerID, job.ID, job.Function)

	w.mu.Lock()
	w.status = WorkerStatusBusy
	w.currentJob = job
	w.mu.Unlock()

	// Update job status
	job.Status = StatusStarted
	now := time.Now()
	job.StartedAt = &now

	// Send job started event
	w.adapter.sendEvent(JobEvent{
		JobID:     job.ID,
		OldStatus: StatusQueued,
		NewStatus: StatusStarted,
		Timestamp: now,
		Worker:    w.config.Name,
	})

	// Create job context with timeout
	jobCtx := w.ctx
	if w.config.JobTimeout > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(w.ctx, w.config.JobTimeout)
		defer cancel()
	}

	// Execute the job
	result, err := w.executeJob(jobCtx, job)

	finishedAt := time.Now()
	job.FinishedAt = &finishedAt

	if err != nil {
		// Job failed
		job.Status = StatusFailed
		job.Error = err.Error()

		log.Printf("[Worker %s-%d] Job failed: %s - %v",
			w.config.Name, workerID, job.ID, err)

		// Handle retry logic
		w.handleJobFailure(job, err)

		// Update stats
		w.adapter.updateQueueStats(job.Queue, 0, -1, 0, 1)
	} else {
		// Job succeeded
		job.Status = StatusFinished
		job.Result = result

		log.Printf("[Worker %s-%d] Job completed: %s",
			w.config.Name, workerID, job.ID)

		// Update stats
		w.adapter.updateQueueStats(job.Queue, 0, -1, 1, 0)
	}

	// Send job finished event
	w.adapter.sendEvent(JobEvent{
		JobID:     job.ID,
		OldStatus: StatusStarted,
		NewStatus: job.Status,
		Timestamp: finishedAt,
		Worker:    w.config.Name,
		Error:     job.Error,
	})

	// Update worker state
	w.mu.Lock()
	w.status = WorkerStatusIdle
	w.currentJob = nil
	w.lastSeen = time.Now()
	w.mu.Unlock()

	atomic.AddInt64(&w.jobsProcessed, 1)
}

// executeJob executes the actual job function
func (w *Worker) executeJob(ctx context.Context, job *Job) (interface{}, error) {
	// Get the registered function
	w.adapter.mu.RLock()
	jobFunc, exists := w.adapter.functions[job.Function]
	w.adapter.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("function not registered: %s", job.Function)
	}

	// Execute the function
	return jobFunc.Execute(ctx, job.Args, job.Kwargs)
}

// handleJobFailure handles job failures and retry logic
func (w *Worker) handleJobFailure(job *Job, err error) {
	if job.RetryPolicy == nil || !job.RetryPolicy.RetryOnFailure {
		// No retry policy, move to failed queue
		w.moveToFailedQueue(job)
		return
	}

	// Get current retry count from headers
	retryCount := 0
	if job.Meta != nil {
		if count, ok := job.Meta["retry_count"].(int); ok {
			retryCount = count
		}
	}

	if retryCount >= job.RetryPolicy.MaxRetries {
		// Max retries exceeded, move to failed queue
		log.Printf("[Worker %s] Job %s exceeded max retries (%d), moving to failed queue",
			w.config.Name, job.ID, job.RetryPolicy.MaxRetries)
		w.moveToFailedQueue(job)
		return
	}

	// Calculate retry delay
	delay := w.calculateRetryDelay(job.RetryPolicy, retryCount)

	// Schedule retry
	retryCount++
	if job.Meta == nil {
		job.Meta = make(map[string]interface{})
	}
	job.Meta["retry_count"] = retryCount
	job.Meta["retry_reason"] = err.Error()
	job.Status = StatusQueued

	retryAt := time.Now().Add(delay)
	log.Printf("[Worker %s] Scheduling retry %d/%d for job %s in %v",
		w.config.Name, retryCount, job.RetryPolicy.MaxRetries, job.ID, delay)

	w.adapter.EnqueueAt(job, retryAt)
}

// calculateRetryDelay calculates the delay for the next retry
func (w *Worker) calculateRetryDelay(policy *RetryPolicy, retryCount int) time.Duration {
	switch policy.BackoffStrategy {
	case BackoffFixed:
		if len(policy.RetryIntervals) > 0 {
			return policy.RetryIntervals[0]
		}
		return 30 * time.Second

	case BackoffLinear:
		baseDelay := 30 * time.Second
		if len(policy.RetryIntervals) > 0 {
			baseDelay = policy.RetryIntervals[0]
		}
		return time.Duration(retryCount+1) * baseDelay

	case BackoffExponential:
		baseDelay := 30 * time.Second
		if len(policy.RetryIntervals) > 0 {
			baseDelay = policy.RetryIntervals[0]
		}
		multiplier := 1 << retryCount // 2^retryCount
		return time.Duration(multiplier) * baseDelay

	case BackoffCustom:
		if len(policy.RetryIntervals) > retryCount {
			return policy.RetryIntervals[retryCount]
		}
		// Fallback to last interval
		if len(policy.RetryIntervals) > 0 {
			return policy.RetryIntervals[len(policy.RetryIntervals)-1]
		}
		return 30 * time.Second

	default:
		return 30 * time.Second
	}
}

// moveToFailedQueue moves a job to the failed queue
func (w *Worker) moveToFailedQueue(job *Job) {
	originalQueue := job.Queue
	job.Queue = "failed"
	job.Status = StatusFailed

	if job.Meta == nil {
		job.Meta = make(map[string]interface{})
	}
	job.Meta["original_queue"] = originalQueue
	job.Meta["failed_at"] = time.Now()

	// Re-enqueue to failed queue
	if err := w.adapter.Enqueue(job); err != nil {
		log.Printf("[Worker %s] Failed to move job %s to failed queue: %v",
			w.config.Name, job.ID, err)
	}
}

// heartbeat sends periodic heartbeat to update last seen time
func (w *Worker) heartbeat() {
	defer w.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.mu.Lock()
			w.lastSeen = time.Now()
			w.mu.Unlock()
		case <-w.ctx.Done():
			return
		}
	}
}
