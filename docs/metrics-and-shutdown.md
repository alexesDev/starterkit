# Metrics and shutdown

## Two kinds of number

Counters are incremented by use cases; gauges are read from the database by a
background loop. That split is the whole design.

A counter is something that happened, so the code that made it happen owns it,
and it declares that the same way it declares everything else:

```go
// internal/case/banuser/resolve.go
type Env interface {
	// ...
	MetricsIncreaseUserBanned(ctx context.Context)
}
```

`app` embeds `*metrics.Metrics`, so the method is promoted and the contract is
satisfied for free — the same mechanism any service object uses, see
[env-pattern.md](env-pattern.md). The context argument is unused today; it is
there so the call site looks like every other `env.` call and so a future
implementation can read from it without changing every caller.

A gauge is a fact about the world right now. Nobody increments a row count — it
would drift the first time the purge job deleted something. Instead
`metrics.Metrics.Update` asks:

```go
type Env interface {
	TableRowCounts(ctx context.Context) (map[string]int64, error)
}
```

which produces one series per table:

```
starterkit_table_rows{table="audit_log"} 6
starterkit_table_rows{table="users"} 2
```

A sweep rather than a hand-picked gauge per table, for one reason: a table added
next month is covered without anyone remembering to add a metric — and "nobody
noticed this table growing unboundedly" is exactly what the metric exists to
prevent. `schema_migrations` comes along for free and is genuinely useful: it
reveals a partially-migrated instance.

`Update` resets the vector before it writes, so a dropped table stops reporting
rather than freezing at its last value. `internal/metrics/metrics_test.go` pins
that.

`count(*)` is a full scan in SQLite — there is no stored row count — but it
measures at about 15ms per million rows, so on the metrics interval it is free.
It reads through the read pool, so it never contends with an open write
transaction.

This is the one place SQL is written by hand instead of generated: sqlc only
generates for statements whose tables are known at build time, and the table
list here comes from `pragma_table_list` at runtime. See
`internal/db/tablestats.go`. `main.go` runs `Metrics.Run` in a goroutine that
ends when the process context is cancelled. A failed update is logged and the
loop continues — one transient read must not silently end monitoring for the
life of the process.

## The counters

| Metric | Moved by |
|---|---|
| `starterkit_sign_ins_total` | a token with a newer `iat` than the row remembers |
| `starterkit_admin_denied_total{operation}` | every refused crossing of the admin boundary |
| `starterkit_users_banned_total` | `banuser` |
| `starterkit_admin_changed_total{action}` | `makeadmin` and `removeadmin` |
| `starterkit_jobs_finished_total{job,outcome}` | `internal/jobs.Register`, around every handler |
| `starterkit_notifications_total{kind}` | `internal/notifier` |

An attempt and an effect are separate facts, and both are recorded. Two
`mutation:removeAdmin` rows in the audit log next to
`starterkit_admin_changed_total{action="remove"} 1` says the first attempt was
refused — see [behavioural-testing.md](behavioural-testing.md).

## Why /metrics has its own listener

`STARTERKIT_METRICS_ADDR` defaults to `127.0.0.1:7302`, separate from the app.

Putting `/metrics` on the public entrypoint would put it behind Traefik's
forward-auth, and a Prometheus scraper has no session. The alternatives are
punching an auth hole in the gate or giving the scraper credentials; a separate
loopback listener is simpler than both and exposes less. Scrape it over the
private network, or from a sidecar.

Metrics are not free of information: the row counts describe the installation.
Treat the endpoint as internal. `scripts/refuse_system_paths.sh` asserts the
public origin does not serve it.

## Zero-downtime shutdown

Stopping a server is not the same as draining one. On SIGTERM:

```
ready.Store(false)        // /readyz starts returning 503 immediately
sleep ShutdownDelay       // the load balancer notices and deregisters us
srv.Shutdown(timeout)     // in-flight requests get their window to finish
```

`ListenAndServe` returns the *instant* `Shutdown` is called, so `run` waits on a
channel closed by the shutdown goroutine before returning. Without that wait the
deferred `conns.Close()` fires immediately and the database is pulled out from
under requests that are still running — the timeout would be configured and
never honoured.

Both durations live in `internal/appconfig`:

| Setting | Default | Meaning |
|---|---|---|
| `STARTERKIT_SHUTDOWN_DELAY` | `0` | how long to keep serving after readiness starts failing |
| `STARTERKIT_SHUTDOWN_TIMEOUT` | `15s` | how long in-flight requests get to finish |

The delay defaults to zero because in development nothing is watching `/readyz`
and waiting would only slow `make dev` down. **In production set it to more than
your load balancer's health-check interval times its failure threshold.** Skip
it and a rolling deploy drops whatever was in flight when the signal arrived:
the balancer is still sending traffic to a process that has stopped accepting.

`/healthz` and `/readyz` — both under `appconfig.BasePath`, default
`/_system/starterkit` — answer different questions and are not interchangeable.
`/healthz` says the process is alive; restart it if this fails. `/readyz` says
send it traffic; during a drain the process is perfectly healthy and must not
receive anything new.

Background work is cancelled through the same context: the job runner stops
receiving, the cron scheduler stops ticking, and the metrics loop returns. A job
that was mid-flight is redelivered by goqite, which is why job handlers have to
be safe to run twice — see [jobs-and-cron.md](jobs-and-cron.md).

`e2e/drain.spec.ts` drives the whole sequence against a real container.
