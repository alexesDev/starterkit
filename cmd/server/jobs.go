package main

import (
	"context"
	"encoding/json"
	"fmt"

	"maragu.dev/goqite"
	goqitejobs "maragu.dev/goqite/jobs"

	"starterkit/internal/case/notifyuserbanned"
	"starterkit/internal/case/purgeauditlog"
	"starterkit/internal/jobs"
)

const (
	jobPurgeAuditLog    = "purge-audit-log"
	jobNotifyUserBanned = "notify-user-banned"
)

func (a *app) RegisterJob(id string, handler func(ctx context.Context, body []byte) error) {
	a.jobRunner.Register(id, goqitejobs.Func(handler))
}

// EnqueueJob writes into the request's transaction when there is one, so an
// enqueue and the row it is about commit or roll back together.
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
	return a.EnqueueJob(ctx, jobNotifyUserBanned, notifyuserbanned.Input{UserID: userID}, 0)
}

// registerJobs is the compile-time proof of wiring: jobs.Register captures a
// typed env, so a job whose Env `app` does not satisfy will not build.
func (a *app) registerJobs() {
	jobs.Register(a, jobPurgeAuditLog, 0, func(ctx context.Context, _ struct{}) error {
		return a.WithTransaction(ctx, func(txCtx context.Context, env *app) error {
			return purgeauditlog.Resolve(txCtx, env)
		})
	})

	jobs.Register(a, jobNotifyUserBanned, 0, func(ctx context.Context, params notifyuserbanned.Input) error {
		return notifyuserbanned.Resolve(ctx, a, params)
	})
}
