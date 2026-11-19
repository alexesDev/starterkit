package main

import (
	"context"

	"starterkit/internal/cronjobs"
)

func (a *app) registerCronJobs() error {
	err := a.scheduleJob(jobPurgeAuditLog, a.config.Audit.PurgeSpec)
	if err != nil {
		return err
	}

	if !a.config.Backup.Enabled() {
		return nil
	}

	return a.scheduleJob(jobBackupDatabase, a.config.Backup.Spec)
}

func (a *app) scheduleJob(id, spec string) error {
	return a.cronRunner.Add(cronjobs.Job{
		ID:   id,
		Spec: spec,
		Run: func(ctx context.Context) error {
			return a.EnqueueJob(ctx, id, struct{}{}, 0)
		},
	})
}
