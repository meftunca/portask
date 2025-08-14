package rq

import (
	"context"
	"time"
)

// Job represents a Redis Queue compatible job
type Job struct {
	ID          string                 `json:"id"`
	Function    string                 `json:"function"`
	Args        []interface{}          `json:"args,omitempty"`
	Kwargs      map[string]interface{} `json:"kwargs,omitempty"`
	Queue       string                 `json:"queue"`
	Priority    Priority               `json:"priority"`
	Timeout     time.Duration          `json:"timeout"`
	RetryPolicy *RetryPolicy           `json:"retry_policy,omitempty"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	FinishedAt  *time.Time             `json:"finished_at,omitempty"`
	Status      JobStatus              `json:"status"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Progress    *JobProgress           `json:"progress,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

// Priority levels for jobs
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	default:
		return "normal"
	}
}

// JobStatus represents the current status of a job
type JobStatus string

const (
	StatusQueued    JobStatus = "queued"
	StatusStarted   JobStatus = "started"
	StatusFinished  JobStatus = "finished"
	StatusFailed    JobStatus = "failed"
	StatusCanceled  JobStatus = "canceled"
	StatusScheduled JobStatus = "scheduled"
	StatusDeferred  JobStatus = "deferred"
)

// RetryPolicy defines how jobs should be retried on failure
type RetryPolicy struct {
	MaxRetries      int             `json:"max_retries"`
	RetryIntervals  []time.Duration `json:"retry_intervals,omitempty"`
	BackoffStrategy BackoffStrategy `json:"backoff_strategy"`
	RetryOnFailure  bool            `json:"retry_on_failure"`
}

// BackoffStrategy defines the retry backoff behavior
type BackoffStrategy string

const (
	BackoffLinear      BackoffStrategy = "linear"
	BackoffExponential BackoffStrategy = "exponential"
	BackoffFixed       BackoffStrategy = "fixed"
	BackoffCustom      BackoffStrategy = "custom"
)

// JobProgress tracks job execution progress
type JobProgress struct {
	Current     int64     `json:"current"`
	Total       int64     `json:"total"`
	Description string    `json:"description,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// JobFunction represents a function that can be executed as a job
type JobFunction interface {
	Execute(ctx context.Context, args []interface{}, kwargs map[string]interface{}) (interface{}, error)
	Name() string
}

// WorkerConfig defines worker configuration
type WorkerConfig struct {
	Name            string        `json:"name"`
	Queues          []string      `json:"queues"`
	Concurrency     int           `json:"concurrency"`
	PollInterval    time.Duration `json:"poll_interval"`
	JobTimeout      time.Duration `json:"job_timeout"`
	GracefulTimeout time.Duration `json:"graceful_timeout"`
	MaxJobs         int           `json:"max_jobs"`
}

// QueueConfig defines queue-specific configuration
type QueueConfig struct {
	Name       string        `json:"name"`
	MaxSize    int           `json:"max_size"`
	DefaultTTL time.Duration `json:"default_ttl"`
	Priority   Priority      `json:"priority"`
	Persistent bool          `json:"persistent"`
}

// RQServer defines the main RQ server interface
type RQServer interface {
	// Job Management
	Enqueue(job *Job) error
	EnqueueAt(job *Job, at time.Time) error
	EnqueueIn(job *Job, delay time.Duration) error
	GetJob(jobID string) (*Job, error)
	CancelJob(jobID string) error
	RequeueJob(jobID string) error

	// Queue Management
	CreateQueue(config QueueConfig) error
	DeleteQueue(name string) error
	PurgeQueue(name string) error
	GetQueueLength(name string) (int, error)
	GetQueueJobs(name string, offset, limit int) ([]*Job, error)

	// Worker Management
	StartWorker(config WorkerConfig) error
	StopWorker(name string) error
	GetWorkers() ([]WorkerInfo, error)

	// Monitoring
	GetStats() (*RQStats, error)
	GetJobStats(queueName string) (*JobStats, error)

	// Function Registration
	RegisterFunction(name string, fn JobFunction) error
	UnregisterFunction(name string) error

	// Lifecycle
	Start() error
	Stop() error
	Health() error
}

// WorkerInfo contains information about a worker
type WorkerInfo struct {
	Name          string    `json:"name"`
	Queues        []string  `json:"queues"`
	Status        string    `json:"status"`
	CurrentJob    *Job      `json:"current_job,omitempty"`
	JobsProcessed int64     `json:"jobs_processed"`
	StartedAt     time.Time `json:"started_at"`
	LastSeen      time.Time `json:"last_seen"`
}

// RQStats contains overall RQ statistics
type RQStats struct {
	TotalJobs    int64                `json:"total_jobs"`
	QueuedJobs   int64                `json:"queued_jobs"`
	StartedJobs  int64                `json:"started_jobs"`
	FinishedJobs int64                `json:"finished_jobs"`
	FailedJobs   int64                `json:"failed_jobs"`
	Workers      int                  `json:"workers"`
	Queues       int                  `json:"queues"`
	QueueStats   map[string]*JobStats `json:"queue_stats"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

// JobStats contains job statistics for a queue
type JobStats struct {
	QueueName    string     `json:"queue_name"`
	Total        int64      `json:"total"`
	Queued       int64      `json:"queued"`
	Started      int64      `json:"started"`
	Finished     int64      `json:"finished"`
	Failed       int64      `json:"failed"`
	Scheduled    int64      `json:"scheduled"`
	OldestQueued *time.Time `json:"oldest_queued,omitempty"`
	NewestQueued *time.Time `json:"newest_queued,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// JobEvent represents job status changes
type JobEvent struct {
	JobID     string    `json:"job_id"`
	OldStatus JobStatus `json:"old_status"`
	NewStatus JobStatus `json:"new_status"`
	Timestamp time.Time `json:"timestamp"`
	Worker    string    `json:"worker,omitempty"`
	Error     string    `json:"error,omitempty"`
}
