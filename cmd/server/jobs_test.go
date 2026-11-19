package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"maragu.dev/goqite"
	goqitejobs "maragu.dev/goqite/jobs"

	"starterkit/internal/audit"
	"starterkit/internal/logger"
)

func enqueueNamed(t *testing.T, a *app, name string) {
	t.Helper()

	_, err := goqitejobs.Create(t.Context(), a.queue, name, goqite.Message{Body: []byte("{}")})
	require.NoError(t, err)
}

func TestAJobNobodyRegisteredIsSetAsideBeforeTheRunnerStarts(t *testing.T) {
	a := newTestApp(t)
	a.RegisterJob(jobPurgeAuditLog, func(context.Context, []byte) error {
		return nil
	})

	enqueueNamed(t, a, "a-job-a-deploy-renamed")

	require.NoError(t, a.setAsideUnregisteredJobs(t.Context()))

	dead, err := a.DeadJobs(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(1), dead.SetAside)

	stop := a.startJobRunner()
	time.Sleep(50 * time.Millisecond)
	stop()

	msg, err := a.queue.Receive(t.Context())
	require.NoError(t, err)
	require.Nil(t, msg, "the set-aside job is still in the queue the runner reads")
}

func TestARegisteredJobStaysQueued(t *testing.T) {
	a := newTestApp(t)
	a.RegisterJob(jobPurgeAuditLog, func(context.Context, []byte) error {
		return nil
	})

	enqueueNamed(t, a, jobPurgeAuditLog)

	require.NoError(t, a.setAsideUnregisteredJobs(t.Context()))

	dead, err := a.DeadJobs(t.Context())
	require.NoError(t, err)
	require.Zero(t, dead.SetAside)
}

func TestStoppingTheRunnerWaitsForTheJobInFlight(t *testing.T) {
	a := newTestApp(t)

	started := make(chan struct{})
	finished := make(chan struct{})

	a.RegisterJob(jobPurgeAuditLog, func(ctx context.Context, _ []byte) error {
		close(started)
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
		close(finished)

		return ctx.Err()
	})

	enqueueNamed(t, a, jobPurgeAuditLog)

	stop := a.startJobRunner()
	<-started
	stop()

	select {
	case <-finished:
	default:
		require.Fail(t, "the runner stopped before its job returned")
	}
}

func TestAnAuditFromAJobTransactionDoesNotWaitOnTheWriter(t *testing.T) {
	for _, failing := range []bool{false, true} {
		a := newTestApp(t)

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)

		err := a.WithTransaction(ctx, func(txCtx context.Context, env *app) error {
			env.WriteAudit(txCtx, audit.Entry{Action: "job_did_something"})

			if failing {
				return errTestRollback
			}

			return nil
		})

		cancel()

		if failing {
			require.ErrorIs(t, err, errTestRollback)
		} else {
			require.NoError(t, err)
		}

		count, err := a.Queries.DBCountAuditLog(t.Context())
		require.NoError(t, err)
		require.Equal(t, int64(1), count, "the audit entry is lost when the job fails: %v", failing)
	}
}

type recordingLogger struct {
	logger.Logger

	warned []string
}

func (l *recordingLogger) Warn(msg string, _ ...any) {
	l.warned = append(l.warned, msg)
}

func (l *recordingLogger) Debug(string, ...any) {}

func TestAFailedJobIsLoggedAsAWarning(t *testing.T) {
	log := &recordingLogger{}
	jobs := jobLog{log: log}

	jobs.Info("Running job", "name", jobPurgeAuditLog)
	jobs.Info("Error running job", "name", jobPurgeAuditLog, "error", errors.New("disk full"))

	require.Equal(t, []string{"Error running job"}, log.warned)
}
