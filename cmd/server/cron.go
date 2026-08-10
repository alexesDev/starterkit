package main

import (
	"context"

	"starterkit/internal/cronjobs"
)

// registerCronJobs schedules work; the queue runs it. Keeping the scheduler
// free of the work itself means a slow job cannot delay the next tick of
// anything else, and the job gets the queue's retry behaviour for free.
func (a *app) registerCronJobs() error {
	return a.cronRunner.Add(cronjobs.Job{
		ID:   jobPurgeAuditLog,
		Spec: a.config.AuditPurgeSpec,
		Run: func(ctx context.Context) error {
			return a.EnqueueJob(ctx, jobPurgeAuditLog, struct{}{}, 0)
		},
	})
}
