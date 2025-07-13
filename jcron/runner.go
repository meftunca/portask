// runner.go (Güncellenmiş Hali)
package jcron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// managedJob yapısı artık görev seçeneklerini de içeriyor.
type managedJob struct {
	id        string
	schedule  Schedule
	job       Job
	nextRun   time.Time
	retryOpts RetryOptions // Yeniden deneme seçenekleri
}

type Runner struct {
	engine *Engine
	jobs   map[string]*managedJob
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger // Yapılandırılmış logger
}

func NewRunner(logger *slog.Logger) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		engine: New(),
		jobs:   make(map[string]*managedJob),
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}
}

func (r *Runner) AddFuncCron(cronString string, cmd func() error, opts ...JobOption) (string, error) {
	schedule, err := FromCronSyntax(cronString)
	if err != nil {
		return "", err
	}
	return r.AddJob(schedule, JobFunc(cmd), opts...)
}

func (r *Runner) AddJob(schedule Schedule, job Job, opts ...JobOption) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var nextRun time.Time
	var err error

	if schedule.Year != nil && *schedule.Year == "@reboot" {
		nextRun = time.Now()
		schedule.Year = strPtr("1970") // Etkisiz hale getir
		r.logger.Info("özel @reboot görevi eklendi, hemen çalıştırılacak")
	} else {
		nextRun, err = r.engine.Next(schedule, time.Now())
		if err != nil {
			return "", err
		}
	}

	id := uuid.New().String()
	mJob := &managedJob{
		id:       id,
		schedule: schedule,
		job:      job,
		nextRun:  nextRun,
	}

	for _, opt := range opts {
		opt(mJob)
	}

	r.jobs[id] = mJob
	if schedule.Year == nil || (schedule.Year != nil && *schedule.Year != "1970") {
		r.logger.Info("yeni görev eklendi", "job_id", id, "next_run", nextRun.Format(time.RFC3339))
	}
	return id, nil
}

func (r *Runner) RemoveJob(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.jobs[id]; exists {
		delete(r.jobs, id)
		r.logger.Info("görev kaldırıldı", "job_id", id)
	}
}

func (r *Runner) Start() {
	r.logger.Info("runner başlatıldı")
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case now := <-ticker.C:
				r.runPending(now)
			case <-r.ctx.Done():
				ticker.Stop()
				r.logger.Info("runner durduruldu")
				return
			}
		}
	}()
}

func (r *Runner) runPending(now time.Time) {
	r.mu.RLock()
	jobsToRun := make([]*managedJob, 0)
	for _, job := range r.jobs {
		if job.nextRun.Before(now) || job.nextRun.Equal(now) {
			jobsToRun = append(jobsToRun, job)
		}
	}
	r.mu.RUnlock()

	for _, job := range jobsToRun {
		go r.runJob(job)

		r.mu.Lock()
		if mJob, exists := r.jobs[job.id]; exists {
			mJob.nextRun, _ = r.engine.Next(mJob.schedule, now)
		}
		r.mu.Unlock()
	}
}

func (r *Runner) runJob(job *managedJob) {
	defer func() {
		if rec := recover(); rec != nil {
			r.logger.Error("görevde panic yaşandı!", "job_id", job.id, "panic", rec)
		}
	}()

	r.logger.Info("görev tetiklendi", "job_id", job.id)

	for attempt := 0; attempt <= job.retryOpts.MaxRetries; attempt++ {
		err := job.job.Run()
		if err == nil {
			r.logger.Info("görev başarıyla tamamlandı", "job_id", job.id)
			return
		}

		if attempt < job.retryOpts.MaxRetries {
			r.logger.Warn(
				"görev hata verdi, yeniden denenecek",
				"job_id", job.id,
				"attempt", fmt.Sprintf("%d/%d", attempt+1, job.retryOpts.MaxRetries),
				"delay", job.retryOpts.Delay,
				"error", err.Error(),
			)
			time.Sleep(job.retryOpts.Delay)
		} else {
			r.logger.Error(
				"görev tüm denemelere rağmen başarısız oldu",
				"job_id", job.id,
				"error", err.Error(),
			)
		}
	}
}

func (r *Runner) Stop() {
	r.cancel()
}
