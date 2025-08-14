package rq

import (
	"context"
	"log"
	"sync"
	"time"
)

// JobScheduler handles scheduled and delayed jobs
type JobScheduler struct {
	scheduledJobs map[string]*ScheduledJob
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	ticker        *time.Ticker
}

// ScheduledJob represents a job scheduled for future execution
type ScheduledJob struct {
	Job         *Job
	ScheduledAt time.Time
	Adapter     *RQAdapter
}

// NewJobScheduler creates a new job scheduler
func NewJobScheduler() *JobScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	scheduler := &JobScheduler{
		scheduledJobs: make(map[string]*ScheduledJob),
		ctx:           ctx,
		cancel:        cancel,
		ticker:        time.NewTicker(1 * time.Second), // Check every second
	}

	go scheduler.run()

	return scheduler
}

// Schedule schedules a job for future execution
func (s *JobScheduler) Schedule(job *Job, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scheduledJob := &ScheduledJob{
		Job:         job,
		ScheduledAt: at,
	}

	s.scheduledJobs[job.ID] = scheduledJob

	log.Printf("[Scheduler] Scheduled job %s (%s) for %v",
		job.ID, job.Function, at.Format(time.RFC3339))

	return nil
}

// Cancel cancels a scheduled job
func (s *JobScheduler) Cancel(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.scheduledJobs[jobID]; exists {
		delete(s.scheduledJobs, jobID)
		log.Printf("[Scheduler] Cancelled scheduled job: %s", jobID)
		return nil
	}

	return nil
}

// GetScheduledJobs returns all scheduled jobs
func (s *JobScheduler) GetScheduledJobs() []*ScheduledJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*ScheduledJob, 0, len(s.scheduledJobs))
	for _, job := range s.scheduledJobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// Stop stops the scheduler
func (s *JobScheduler) Stop() {
	s.cancel()
	s.ticker.Stop()
}

// run is the main scheduler loop
func (s *JobScheduler) run() {
	for {
		select {
		case <-s.ticker.C:
			s.processScheduledJobs()
		case <-s.ctx.Done():
			return
		}
	}
}

// processScheduledJobs processes jobs that are ready to run
func (s *JobScheduler) processScheduledJobs() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	var jobsToRun []*ScheduledJob

	// Find jobs ready to run
	for jobID, scheduledJob := range s.scheduledJobs {
		if scheduledJob.ScheduledAt.Before(now) || scheduledJob.ScheduledAt.Equal(now) {
			jobsToRun = append(jobsToRun, scheduledJob)
			delete(s.scheduledJobs, jobID)
		}
	}

	// Execute ready jobs
	for _, scheduledJob := range jobsToRun {
		job := scheduledJob.Job
		job.Status = StatusQueued
		job.ScheduledAt = nil

		log.Printf("[Scheduler] Executing scheduled job: %s (%s)",
			job.ID, job.Function)

		// This would typically re-enqueue the job
		// For now, we'll just log it
		// In a real implementation: scheduledJob.Adapter.Enqueue(job)
	}
}
