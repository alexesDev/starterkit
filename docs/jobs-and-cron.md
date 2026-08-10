# Background jobs and cron

Two mechanisms with different jobs: [goqite](https://github.com/maragudk/goqite)
runs work, cron schedules it. Neither needs Redis or a broker — the queue is a
table in the same SQLite file.

## Why the queue lives in the application database

Because an enqueue can then be part of the transaction that caused it:

```go
func (a *app) EnqueueJob(ctx context.Context, id string, payload any, priority int) error {
	// ...
	txEnv := a.txEnvFromCtx(ctx)
	if txEnv != nil {
		_, err = goqitejobs.CreateTx(ctx, txEnv.currentTx, a.queue, id, msg)
	} else {
		_, err = goqitejobs.Create(ctx, a.queue, id, msg)
	}
}
```

Enqueue inside a mutation and the job and the row it is about commit or roll
back together. With an external broker that is a distributed-transaction
problem; here it is free.

The kit demonstrates it end to end. `banuser` declares a port on its `Env`:

```go
EnqueueUserBannedNotice(ctx context.Context, userID int64) error
```

`app` implements it in one line, and because the ban runs inside `runInTx` the
notice is enqueued on the same transaction. A ban that rolls back sends nothing;
a ban that commits cannot fail to send. `cmd/server/tx_test.go` asserts both
halves.

The queue gets its own connection pool, so polling does not wait on the app's
single write connection. SQLite's write lock is still shared, so a poll can wait
behind an open transaction — separate pools, one lock. See
[data-layer.md](data-layer.md).

## Registering a job

`internal/jobs.Register` is the typed seam:

```go
jobs.Register(a, jobNotifyUserBanned, 0, func(ctx context.Context, params notifyuserbanned.Input) error {
	return notifyuserbanned.Resolve(ctx, a, params)
})
```

It registers the handler and returns an enqueue function in one call. The
generic parameter exists only to unmarshal the payload — it is not a disguised
runtime type assertion. The handler closes over a typed `env`, so a job whose
`Env` `app` does not satisfy fails to compile.

A job that writes must open its own transaction — there is no request middleware
out here:

```go
jobs.Register(a, jobPurgeAuditLog, 0, func(ctx context.Context, _ struct{}) error {
	return a.WithTransaction(ctx, func(txCtx context.Context, env *app) (bool, error) {
		err := purgeauditlog.Resolve(txCtx, env)
		return err == nil, err
	})
})
```

Retries, timeouts and the at-least-once guarantee come from goqite:
`MaxReceive: 3`, `Timeout: 5 * time.Minute`, with the timeout extended while a
handler runs. **A handler must be safe to run twice.**

## Cron

`internal/cronjobs` wraps `robfig/cron/v3` with two things it lacks:

- **Overlap suppression.** A tick is dropped if the previous run of the same job
  is still going, rather than letting two copies race for the single write
  connection.
- **Panic recovery**, so one bad job does not take the scheduler down.

```go
func (a *app) registerCronJobs() error {
	return a.cronRunner.Add(cronjobs.Job{
		ID:   jobPurgeAuditLog,
		Spec: a.config.AuditPurgeSpec,
		Run: func(ctx context.Context) error {
			return a.EnqueueJob(ctx, jobPurgeAuditLog, struct{}{}, 0)
		},
	})
}
```

Note what the cron job does: it *enqueues*, it does not work. Keeping the
scheduler free of the work means a slow job cannot delay the next tick of
anything else, and the job gets the queue's retry behaviour for free. That split
is the pattern worth copying.

## The jobs

| Job | What it is |
|---|---|
| `purge-audit-log` | Cron enqueues it on `STARTERKIT_AUDIT_PURGE_SPEC` (default 04:00); the job deletes `audit_log` rows older than `STARTERKIT_AUDIT_RETENTION` (default 30 days). Setting the retention to `0` disables the purge. |
| `notify-user-banned` | Enqueued inside the transaction that writes the ban. The job loads the user and the ban and hands a notice to `internal/notifier`. A user deleted or unbanned between the enqueue and the run is the answer, not a failure. |

`notify-user-banned` is also where the "safe to run twice" requirement earns its
keep: the handler is a read plus a delivery, so a redelivery sends the notice
again rather than corrupting anything. A job that must be exactly-once needs a
row to say it already happened.

## Shutdown

`main.go` builds the runner context from `signal.NotifyContext`, so SIGINT and
SIGTERM cancel the job runner and stop the cron scheduler, while the HTTP server
gets its drain window. An in-flight job finishes or times out and is
redelivered. See [metrics-and-shutdown.md](metrics-and-shutdown.md).
