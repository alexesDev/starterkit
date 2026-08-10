# Architecture

## The shape

```
browser
  │  $mol SPA (assets/ui) — one GraphQL endpoint, no REST
  ▼
traefik ──forwardAuth──▶ oauth2-proxy ──▶ dex          (see traefik-forward-auth.md)
  │
  ▼
cmd/server
  ├── http.go        routes, identity from the gate
  ├── auth.go        the identity lookup and the audit writer
  ├── app.go         the app struct — the IO spine
  ├── tx.go          transaction scoping
  ├── commands.go    one command method per mutation
  ├── jobs.go        queue registration
  └── cron.go        schedule registration
        │
        ├── internal/graph      thin resolvers, one line per case
        ├── internal/case/*     the business logic, one package per use case
        ├── internal/notifier   a service object, embedded into app
        ├── internal/dbtime     the type every time column binds to
        └── internal/db         sqlc-generated queries, split read/write
                │
                ▼
        data.sqlite3 (WAL) — application tables + the job queue
```

## The one idea worth internalising

Every use case declares the *narrow* interface it needs and nothing more:

```go
// internal/case/banuser/resolve.go
type Env interface {
	DBGetUserByID(ctx context.Context, id int64) (db.User, error)
	DBInsertUserBan(ctx context.Context, arg db.DBInsertUserBanParams) (db.UserBan, error)
	CurrentUserID(ctx context.Context) *int64
	EnqueueUserBannedNotice(ctx context.Context, userID int64) error
	MetricsIncreaseUserBanned(ctx context.Context)
}

func Resolve(ctx context.Context, env Env, input Input) (Payload, error)
```

A single `app` struct in `cmd/server` satisfies all of them at once, by Go's
structural typing. It passes *itself* wherever an `Env` is wanted.

Nothing registers anything. There is no container, no reflection, no
`wire.Build`. Passing `a` to `graph.NewHandler` in `cmd/server/http.go` is the
entire dependency injection system, and it runs at compile time: `*app`
satisfies `graph.Env` there or the build fails.

The full reasoning, including how a service object plugs into the same
mechanism and the failure modes the pattern has in practice, is in
[env-pattern.md](env-pattern.md).

## Where to put things

| You are adding | Put it in |
|---|---|
| A new screen | `assets/ui/<name>/` + its `.graphql` files |
| A new query or mutation | `internal/graph/schema.graphqls`, then a case package |
| Business logic | `internal/case/<verb><noun>/resolve.go` |
| A SQL query | `queries.read.sql` or `queries.write.sql`, then `make sqlc` |
| A table | `make db-new name=...`, then `make db-up` |
| A background job | `internal/case/<name>/` + a line in `cmd/server/jobs.go` |
| A scheduled job | a line in `cmd/server/cron.go` |
| An external service client | `internal/<service>/`, then embed it into `app` |

## Conventions that are load-bearing

**Infrastructure failure is an `error`. Domain failure is a payload.**

```go
// the database is unreachable — the caller cannot fix this, and it is a 500
return nil, fmt.Errorf("failed to load user: %w", err)

// the last admin cannot be removed — a normal answer the UI renders
return &model.ErrorPayload{Message: "the last admin cannot be removed"}, nil
```

Nothing enforces this. It is the convention that makes the GraphQL union types
work, and it is why a rejected input does not page anyone.

**Reads and writes are different types.** `db.Queries` has only `select`
methods; `db.WriteQueries` has the rest. Code holding a read handle cannot
write, and the compiler is what says so. See [data-layer.md](data-layer.md).

**A command owns its transaction.** Each mutation is a command method on `app`
that decides what is atomic: `runInTx` around the case for those that only
write, and no transaction at all for one that has to call a third party before
its single insert. Resolvers never see it. See [graphql.md](graphql.md).

**The admin boundary is one object.** `Query.admin` and `Mutation.admin`
resolve a marker struct after checking the caller is an admin, so every field
beneath them is gated by construction rather than by remembering. See
[auth-dex.md](auth-dex.md).

**Timestamps are integer unix seconds, `dbtime.Stamp` in Go, and a `Timestamp`
object in GraphQL.** Formatting happens on the server, declared once for the
whole API. See [rules.md](rules.md).

## The supporting stack

Dex, oauth2-proxy and Traefik run on docker compose (`docker/compose.yml`).
There is one way to bring an environment up: `make up` for dev,
`scripts/e2e.sh` for e2e, both driving the same compose file through
`scripts/stack.sh`, which picks the project name, the address and the ports per
stand.

Each stand has its own Traefik on its own loopback address — dev on
`127.0.0.81`, e2e on `127.0.0.82` — and both hold port 80, because all of
`127.0.0.0/8` is loopback and two processes may share a port on different
addresses. So the stands run at once with no port collision *and* without
sharing a process: taking one down leaves the other serving. Every upstream sits
on a static loopback port, and Traefik's file provider routes them by name
(`docker/traefik/dynamic/`).

Dev Dex keeps its signing keys in the compose project's named volume because
losing them signs everyone out ([auth-dex.md](auth-dex.md)); `make purge`
removes the volume, and the e2e project's copy is purged at the start of every
run.

## What the kit does not have

Deliberately, so that the first project built on it adds them knowingly rather
than inheriting them:

- **No stored credential.** The panel verifies the gate's ID token and holds no
  session, no client secret and no signing key. Nothing in the schema is a
  secret, so there is no sealing layer and no `@sensitive` redaction in the
  audit walker. [graphql.md](graphql.md) says what to add on the day the first
  secret-bearing mutation arrives.
- **No REST.** One GraphQL endpoint, plus `/healthz`, `/readyz` and
  `/settings.js`.
- **No caching layer.** Identity is one indexed read per request; the metrics
  gauges are the only background reads.
