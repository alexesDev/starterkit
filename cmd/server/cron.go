package main

import (
	"context"

	"starterkit/internal/cronjobs"
)

func (a *app) registerCronJobs() error {
	return a.cronRunner.Add(cronjobs.Job{
		ID:   jobPurgeAuditLog,
		Spec: a.config.Audit.PurgeSpec,
		Run: func(ctx context.Context) error {
			return a.EnqueueJob(ctx, jobPurgeAuditLog, struct{}{}, 0)
		},
	})
}
