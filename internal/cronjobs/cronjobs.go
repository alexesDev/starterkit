// Package cronjobs wraps robfig/cron with overlap suppression and panic
// recovery. See docs/jobs-and-cron.md.
package cronjobs

import (
	"context"
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"

	"starterkit/internal/logger"
)

type Job struct {
	ID   string
	Spec string
	Run  func(ctx context.Context) error
}

type Runner struct {
	cron    *cron.Cron
	log     logger.Logger
	ctx     context.Context
	mu      sync.Mutex
	running map[string]bool
}

func New(ctx context.Context, log logger.Logger) *Runner {
	return &Runner{
		cron:    cron.New(),
		log:     log,
		ctx:     ctx,
		running: map[string]bool{},
	}
}

func (r *Runner) Add(job Job) error {
	_, err := r.cron.AddFunc(job.Spec, func() { r.run(job) })
	if err != nil {
		return fmt.Errorf("failed to schedule cron job %s: %w", job.ID, err)
	}

	return nil
}

func (r *Runner) run(job Job) {
	r.mu.Lock()

	if r.running[job.ID] {
		r.mu.Unlock()
		r.log.Warn("cron job still running, skipping tick", "job", job.ID)

		return
	}

	r.running[job.ID] = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.running, job.ID)
		r.mu.Unlock()
	}()

	defer func() {
		recovered := recover()
		if recovered != nil {
			r.log.Error("cron job panicked", "job", job.ID, "panic", recovered)
		}
	}()

	err := job.Run(r.ctx)
	if err != nil {
		r.log.Error("cron job failed", "job", job.ID, "error", err)
	}
}

func (r *Runner) Start() {
	r.cron.Start()
}

func (r *Runner) Stop() {
	<-r.cron.Stop().Done()
}
