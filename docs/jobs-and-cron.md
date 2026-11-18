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
| `notify-user-banned` | Enqueued inside the transaction that writes the ban, with an idempotency key minted by that enqueue. The job loads the user and the ban and hands the notice and the key to `internal/notifier`. A user deleted or unbanned between the enqueue and the run is the answer, not a failure. |

## A call that creates something elsewhere carries an idempotency key

A handler that only reads and writes this database is safe to run twice by
being careful. A handler that asks another system to *create* something — send
a message, open a ticket, charge a card — is not, because the other system
remembers the first call even when this process does not. The kit's
`notify-user-banned` job is the worked example, and four things make it safe:

**The key is born with the intent, in the same transaction.** The enqueue mints
it, and it travels in the job's payload:

```go
func (a *app) EnqueueUserBannedNotice(ctx context.Context, userID int64) error {
	notice := notifyuserbanned.Input{
		UserID: userID,
		Key:    notifier.NewIdempotencyKey(),
	}

	return a.EnqueueJob(ctx, jobNotifyUserBanned, notice, 0)
}
```

Because the payload is a row in the same SQLite file, the key commits with the
ban or rolls back with it, and every redelivery of that row reads the same key.
A key minted inside the handler would be a new key on every run, which is the
same as no key. `cmd/server/tx_test.go` redelivers a message and asserts the key
did not change.

**The key goes to the other side.** With `STARTERKIT_NOTIFIER_URL` set, the
notifier posts each notice to that relay with an `Idempotency-Key` header, and
a relay that honours the header answers a repeated key with the original result
instead of a second notice. A relay that does not honour it needs the key put
wherever it does dedupe — a message id, a reference field — and if there is no
such place, the call is not safe to retry and the job must say so. With the URL
empty the notifier writes the notice and its key to the log, which is how the
kit runs with nothing configured.

**A timeout is "unknown", not "failed".** A post that timed out may have been
delivered — the request arrived, the response was lost. So the notifier returns
an error that says the outcome is unknown, the handler returns it, and goqite
redelivers the same message after its timeout, carrying the same key. What the
handler must not do is treat the timeout as a clean failure and create the
thing again under a new key, or roll back state that says the intent exists.
`internal/notifier/notifier_test.go` runs a relay that accepts the notice and
never answers, then retries, and asserts the relay holds one notice, not two.

**A refusal is an error too, and its body is not logged whole.** A conflict
(another attempt with the key is still in flight), a not-found or a rate limit
carries no body; any other refusal carries one line of it, capped — see the rule in
[rules.md](rules.md).

Goqite's `MaxReceive: 3` bounds the retries. A notice whose relay times out on
every attempt is dropped after the third, and the job's failure counter in
`starterkit_jobs_finished_total` is where that shows. A job that must never be
dropped needs a row that records the outcome, not more retries.

## Shutdown

`main.go` builds the runner context from `signal.NotifyContext`, so SIGINT and
SIGTERM cancel the job runner and stop the cron scheduler, while the HTTP server
gets its drain window. An in-flight job finishes or times out and is
redelivered. See [metrics-and-shutdown.md](metrics-and-shutdown.md).
