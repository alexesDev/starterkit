package main

import (
	"context"
	"encoding/json"
	"fmt"

	"maragu.dev/goqite"
	goqitejobs "maragu.dev/goqite/jobs"

	"starterkit/internal/case/deadletterjobs"
	"starterkit/internal/case/notifyuserbanned"
	"starterkit/internal/case/purgeauditlog"
	"starterkit/internal/db"
	"starterkit/internal/jobs"
	"starterkit/internal/metrics"
	"starterkit/internal/notifier"
)

const (
	jobPurgeAuditLog    = "purge-audit-log"
	jobNotifyUserBanned = "notify-user-banned"
)

const (
	jobQueue      = "jobs"
	jobDeadQueue  = "jobs-dead"
	jobMaxReceive = 3
)

func (a *app) RegisterJob(id string, handler func(ctx context.Context, body []byte) error) {
	a.jobRunner.Register(id, goqitejobs.Func(handler))
	a.jobNames[id] = struct{}{}
}

func (a *app) JobRegistered(name string) bool {
	_, registered := a.jobNames[name]

	return registered
}

func (a *app) setAsideUnregisteredJobs(ctx context.Context) error {
	input := deadletterjobs.Input{Queue: jobQueue, DeadQueue: jobDeadQueue}

	return a.WithTransaction(ctx, func(txCtx context.Context, env *app) error {
		return deadletterjobs.Resolve(txCtx, env, input)
	})
}

func (a *app) DeadJobs(ctx context.Context) (metrics.DeadJobs, error) {
	exhaustedParams := db.DBCountExhaustedJobsParams{Queue: jobQueue, MaxReceive: jobMaxReceive}

	exhausted, err := a.Queries.DBCountExhaustedJobs(ctx, exhaustedParams)
	if err != nil {
		return metrics.DeadJobs{}, fmt.Errorf("failed to count exhausted jobs: %w", err)
	}

	setAside, err := a.Queries.DBCountQueuedJobs(ctx, jobDeadQueue)
	if err != nil {
		return metrics.DeadJobs{}, fmt.Errorf("failed to count set-aside jobs: %w", err)
	}

	return metrics.DeadJobs{Exhausted: exhausted, SetAside: setAside}, nil
}

func (a *app) EnqueueJob(ctx context.Context, id string, payload any, priority int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal %s payload: %w", id, err)
	}

	msg := goqite.Message{Body: body, Priority: priority}

	txEnv := a.txEnvFromCtx(ctx)
	if txEnv != nil {
		_, err = goqitejobs.CreateTx(ctx, txEnv.currentTx, a.queue, id, msg)
	} else {
		_, err = goqitejobs.Create(ctx, a.queue, id, msg)
	}

	if err != nil {
		return fmt.Errorf("failed to enqueue %s: %w", id, err)
	}

	return nil
}

func (a *app) EnqueueUserBannedNotice(ctx context.Context, userID int64) error {
	notice := notifyuserbanned.Input{
		UserID: userID,
		Key:    notifier.NewIdempotencyKey(),
	}

	return a.EnqueueJob(ctx, jobNotifyUserBanned, notice, 0)
}

func (a *app) registerJobs() {
	jobs.Register(a, jobPurgeAuditLog, func(ctx context.Context, _ struct{}) error {
		return a.WithTransaction(ctx, func(txCtx context.Context, env *app) error {
			return purgeauditlog.Resolve(txCtx, env)
		})
	})

	jobs.Register(a, jobNotifyUserBanned, func(ctx context.Context, params notifyuserbanned.Input) error {
		return notifyuserbanned.Resolve(ctx, a, params)
	})
}
