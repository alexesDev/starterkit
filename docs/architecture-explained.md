# The architecture, explained

[architecture.md](architecture.md) is the map. This page is the walk: what each
layer is, which way the dependencies point, what happens to a request and to a
job from the first byte to the last write, why each choice was made and what it
costs, and how to add the things a project adds in its first month. Every claim
here names the file and the symbol it was checked against, such as
(`cmd/server/main.go`, `run`). References name a symbol, not a line: an edit
elsewhere in the file leaves them pointing at the right code, and when a named
symbol changes, the claims that name it are the ones to re-read.

If you read one other page, read [env-pattern.md](env-pattern.md). If you read
two, add [rules.md](rules.md).

## The layers and the direction of dependencies

```
cmd/server             the composition root: app, commands, tx, jobs, cron, http
   │
   ├──▶ internal/graph          gqlgen schema, thin resolvers, graph.Env
   │       │
   │       └──▶ internal/case/*  one package per use case: Env, Input, Payload, Resolve
   │               │
   │               └──▶ internal/db, internal/dbtime, internal/graph/model,
   │                    internal/notifier (types only)
   │
   ├──▶ internal/db             sqlc output, the read/write split, pools, migrate
   ├──▶ internal/{metrics, notifier, gateauth, cronjobs, jobs, logger, appconfig,
   │              appreq, audit, clientip, dbtime, dbmate/sqlite}
   └──▶ assets                  the embedded $mol bundle and its shell
```

Arrows point at what a package imports. Three properties hold and are worth
keeping:

- **Nothing imports `cmd/server`.** It is `package main`, so nothing can. Every
  other package declares what it needs as an interface and `app` satisfies it.
  `internal/graph` declares `graph.Env` (`internal/graph/resolver.go`, `Env`),
  `internal/metrics` declares `metrics.Env` (`internal/metrics/metrics.go`, `Env`),
  `internal/notifier` declares `notifier.Env`
  (`internal/notifier/notifier.go`, `Env`), `internal/jobs` declares `jobs.Env`
  (`internal/jobs/jobs.go`, `Env`). That is consumer-defined interfaces all the way
  down, and it is why there are no import cycles to break.
- **A case imports no other case.** A case that needs another's effect declares
  a port on its own `Env`. `banuser` wants a notice sent and declares
  `EnqueueUserBannedNotice` (`internal/case/banuser/resolve.go`, `Env`); it never
  imports `notifyuserbanned`.
- **A case does not import `internal/graph`.** It imports `internal/graph/model`
  for the generated `Input` and `Payload` types (the imports of `internal/case/banuser/resolve.go`), which
  is data, not behaviour. `graph` imports the cases, not the other way round.

`internal/graph/model` (generated GraphQL types) and `internal/model` (types the
schema binds to by hand: the admin namespace markers, `ErrNotAdmin`,
`AuthIdentity`, `internal/model/model.go`) are two different packages with
similar names. The first is regenerated; the second is written.

## The request path, end to end

A `banUser` mutation, from the browser to the commit.

**1. The gate.** Traefik's forwardAuth asks oauth2-proxy whether the request
has a session. If not, the browser goes through Dex and back. If it does,
oauth2-proxy sets `Authorization: Bearer <id token>`
(`docker/compose.yml:121`) and Traefik forwards to the panel. See
[traefik-forward-auth.md](traefik-forward-auth.md).

**2. The HTTP server.** `run` builds an `http.Server` with read, write and idle
timeouts and a header cap (`cmd/server/main.go`, `run`). The handler is
`securityHeaders(a.withRequest(mux))` (`cmd/server/http.go`, `app.routes`), so every
response carries the CSP and framing headers (`cmd/server/http.go`, `securityHeaders`) and every request
goes through `withRequest` before routing.

**3. Identity, once per request.** `withRequest` (`cmd/server/http.go`, `app.withRequest`) creates an
`appreq.Request` carrying the client IP, resolved by `clientip.Resolve` against
the trusted proxy list, and puts it in the context. It then calls
`resolveIdentity` (`cmd/server/auth.go`, `app.resolveIdentity`):

- `gateauth.Client.Identity` verifies the ID token against the issuer's JWKS,
  checks the audience, and requires a verified email
  (`internal/gateauth/gateauth.go`, `Client.Identity`). No token is not an error; `/healthz`
  and `/readyz` are served without one.
- One indexed read, `DBGetIdentityByOIDC`, returns the user, whether they are an
  admin, and any ban reason, in a single row (`queries.read.sql`, used at
  `cmd/server/auth.go`, `app.resolveIdentity`). It goes through the read pool.
- A first sign-in runs the `signinbyoidc` case in its own write transaction
  (`cmd/server/auth.go`, `app.SignInByOIDC` and `app.firstSignIn`), which upserts the user and promotes it to
  admin when its email matches the configured bootstrap admin and the `admins`
  table is empty
  (`internal/case/signinbyoidc/resolve.go`).
- A later sign-in is noticed by the token's `iat` advancing, and recorded
  directly on the base write handle (`cmd/server/auth.go`, `app.recordSignIn`).

A banned identity gets a 403 and a stale token a redirect or a 401
(`cmd/server/http.go`, `app.withRequest` and `app.refuseStaleToken`) before any route runs. Admin
rights and bans are read from the database on every request, so revoking
either applies to the next request rather than at the token's expiry.

**4. Routing.** `POST <base>/graphql` is `limitBody(requireIdentity(graph handler))`
(`cmd/server/http.go`, `app.routes`). `limitBody` caps the body at 1 MiB (`cmd/server/http.go`, `limitBody`);
`requireIdentity` refuses a request with no verified user (`cmd/server/http.go`, `app.requireIdentity`). The
GraphQL handler (`internal/graph/handler.go`) takes POST (and answers OPTIONS),
caches parsed queries, and refuses an operation above a fixed complexity of 60.

**5. The admin boundary.** Every real mutation lives under `Mutation.admin`.
Its resolver calls `checkAdmin` (`internal/graph/resolver.go`, `Resolver.checkAdmin`), which reads
the identity `withRequest` already loaded, writes a `denied:` audit entry and
increments a counter on refusal, and for a mutation that is allowed records the
selected fields and the variables as an audit entry. Because this runs when the
namespace object resolves, every field beneath it is gated without each field
remembering to check.

**6. The resolver.** `AdminMutation.banUser` is one line
(`internal/graph/schema.resolvers.go`, `adminMutationResolver.BanUser`):

```go
return r.env(ctx).BanUser(ctx, input)
```

`r.env(ctx)` returns `graph.Env`, which `app` satisfies. The resolver knows
nothing about transactions, SQL or validation.

**7. The command.** `app.BanUser` (`cmd/server/commands.go`, `app.BanUser`) is where the
transaction boundary is chosen:

```go
return runInTx(ctx, a, func(txCtx context.Context, env *app) (banuser.Payload, error) {
	return banuser.Resolve(txCtx, env, input)
})
```

`runInTx` (`cmd/server/commands.go`, `runInTx`) wraps `WithTransaction` (`cmd/server/tx.go`, `app.WithTransaction`),
which refuses to nest, begins a transaction on the single-connection write pool,
and makes a shallow copy of `app` whose `Queries` and `WriteQueries` are bound
to that transaction (`cmd/server/tx.go`, `app.withTxQueries`). The copy is also stored in the context, so
code that only has a `ctx` can still find the open transaction.

**8. The case.** `banuser.Resolve` (`internal/case/banuser/resolve.go`, `Resolve`)
receives the transactional copy as its narrow `Env`. It trims and validates the
input and returns an `ErrorPayload` with a nil error for anything the domain
rejects; reads the user through the transaction; inserts the ban; enqueues the
notice; and increments a counter. It never mentions a transaction.

**9. The enqueue.** `EnqueueUserBannedNotice` is one line on `app`
(`cmd/server/jobs.go`, `app.EnqueueUserBannedNotice`) that calls `EnqueueJob` (`cmd/server/jobs.go`, `app.EnqueueJob`).
`EnqueueJob` finds the open transaction through the context and calls
`goqitejobs.CreateTx` on it, so the queue row is written in the same
transaction as the ban.

**10. Commit or roll back.** `WithTransaction` commits when the callback returns
a nil error and rolls back otherwise (`cmd/server/tx.go`, `app.WithTransaction`). A domain
failure is a nil error, so a case that wrote nothing before deciding it
failed commits nothing.

**11. Errors on the way out.** An infrastructure `error` reaches gqlgen's error
presenter (`internal/graph/error_presenter.go`), which logs the cause with a
random reference and returns only `internal error, ref <hex>` to the caller.
Parser and validator errors and `ErrNotAdmin` pass through unchanged.

**12. Audit, after the transaction.** While a request is in scope,
`WriteAudit` only appends to the request (`cmd/server/auth.go`, `app.WriteAudit`). After the handler
returns, `withRequest` flushes the buffer through the base write handle, outside
any transaction and detached from the request's cancellation (`cmd/server/http.go`, `app.withRequest`; `cmd/server/auth.go`, `app.flushAudit`). So the record of a denied or rolled-back operation survives, and
a client that disconnects cannot take its audit row with it. A job has no
request, so `WithTransaction` carries the same buffer itself and flushes it
after its commit or rollback (`cmd/server/tx.go`, `app.WithTransaction`).

## The job path

**1. Enqueue in the transaction.** Step 9 above: the job row and the row it is
about commit or roll back together. Outside a transaction `EnqueueJob` writes
the queue row on its own (`cmd/server/jobs.go`, `app.EnqueueJob`).

**2. The queue.** The queue is [goqite](https://github.com/maragudk/goqite), a
table in the same SQLite file (`db/migrations/20260810090400_create_goqite.sql`),
read through its own single-connection pool (`internal/db/setup.go`, `Setup`).
`newApp` configures it with `MaxReceive: 3` and a 5-minute visibility timeout,
and a runner that polls every 2 seconds and runs at most 2 jobs at once
(`cmd/server/main.go`, `newApp`).

**3. The handler.** `registerJobs` (`cmd/server/jobs.go`, `app.registerJobs`) registers each job through
`jobs.Register` (`internal/jobs/jobs.go`, `Register`), which unmarshals the JSON payload
into the handler's own typed parameter, runs it, and increments
`jobs_finished_total{job,outcome}`. The handler closes over `a`, so the
compiler checks that `app` satisfies the case's `Env` at the registration site.

**4. The case.** `notifyuserbanned.Resolve` runs against the base `app`, with no
transaction, because it only reads and then delivers
(`internal/case/notifyuserbanned/resolve.go`). A job that writes opens its own:
`purge-audit-log` wraps its case in `a.WithTransaction` (`cmd/server/jobs.go`, `app.registerJobs`).

**5. Retry.** A handler that returns an error leaves its message in the queue,
and goqite hands it out again once the message's timeout expires, up to
`MaxReceive` deliveries in all. While a handler runs, goqite's runner extends
that timeout by 5 seconds every 4 seconds (its default `Extend`), so the
configured 5 minutes is the retry delay only for a handler that fails within its
first few seconds; one that ran longer is retried within about 5 seconds of
failing. A handler that succeeds deletes its message. Delivery is at least once, so **a
handler must be safe to run twice**.

**Cron** sits in front of this, not beside it. `internal/cronjobs` wraps
`robfig/cron/v3` with overlap suppression and panic recovery
(`internal/cronjobs/cronjobs.go`, `Runner.run`). A cron entry's whole body is an enqueue
(`cmd/server/cron.go`, `app.registerCronJobs`), so the scheduler never does the work, a slow job
never delays another tick, and scheduled work inherits the queue's retries.

## Why each choice was made, and what it costs

### Narrow `Env` per case

Each case declares only the methods it calls (`internal/case/banuser/resolve.go`, `Env`). A
reader knows what a case can reach without leaving the file, `moq` generates a
mock of exactly that much, and a test sets three functions rather than an
application.

**Cost:** a method is named three times — on the case's `Env`, on `app` (or
promoted from something embedded in it), and in the mock. Method names share
one flat namespace on `app`, so two services cannot both expose `Get`; name
them for what they do. And the interface is only as narrow as its author keeps
it: nothing stops an `Env` from listing twenty methods.

### `app` as the one IO spine

One struct holds every connection, client and piece of process state
(`cmd/server/app.go`, `app`) and satisfies every `Env` at once. Passing `a` to
`graph.NewHandler` (`cmd/server/http.go`, `app.routes`) and to `jobs.Register` (`cmd/server/jobs.go`, `app.registerJobs`) is the
entire dependency injection, and it runs at compile time.

`app` is split in two on purpose. `app` itself holds only what changes per
transaction — the query handles and `currentTx` — and everything that lives as
long as the process sits behind the `*appState` pointer (`cmd/server/app.go`, `appState`), so the
shallow copy in `withTxQueries` shares it. A mutex, cache or atomic placed
directly on `app` would be silently duplicated into every transaction.

**Cost:** `app` grows with every port, and `cmd/server` is where a project's
glue accumulates. The compiler proves that the wiring exists, not that it is
small. When a service object does real work it belongs in its own package with
its own `Env`, embedded into `appState` (`internal/notifier` is the example),
not as more methods in `cmd/server`.

### The command method as the transaction boundary

The transaction begins and ends in a command method on `app`, never in a
resolver and never in a case (`commands.go`). A case is pure logic against its
`Env`, so it composes inside whatever transaction its caller chose, and a second
write added to a case later is atomic with the first by construction.

**Cost:** a command has to be written per mutation, three lines each. Two
mutations under one `admin` selection are two transactions, not one: gqlgen
resolves the fields beneath the namespace object concurrently, and each command
opens its own transaction on the single writer. A command must call a case's
`Resolve`, never another command, or it gets `ErrNestedTransaction`
(`cmd/server/tx.go`, `ErrNestedTransaction`). And the commit decision is the error, not the payload: a case
that writes and *then* returns an `ErrorPayload` commits what it wrote.

### The read/write query split and the naming rule

`queries.read.sql` holds selects and `queries.write.sql` holds everything else.
sqlc generates both onto `*Queries`, and `make sqlc` then runs
`internal/db/fix_write_queries.sh`, which rebinds the write methods onto
`*WriteQueries` (`Makefile`, target `sqlc`). A function holding `*db.Queries`
cannot call an insert; the compiler says so. The read pool also opens with
`query_only(1)` (`internal/db/setup.go`, `open`), so a write that somehow reached it
would still fail at the database.

The `DB` prefix keeps generated names apart from command methods on `app`, and
`DBGet`/`DBCount`/`DBList` say a read's cardinality.
`internal/db/list_queries.sh`, run by `make lint`, refuses a write in the read
file, a top-level select in the write file, and a misnamed query.

**Cost:** two files to choose between and a post-processing script that bare
`sqlc generate` skips. Running sqlc directly dissolves the split without an
error. The shell checks are text matching, so they catch the common mistakes,
not every one.

### sqlc over an ORM

The SQL you write is the SQL that runs, and the Go types come from the schema
rather than from struct tags. A GraphQL type binds straight onto a generated row
with `@goModel` (`internal/graph/schema.graphqls`), so the common field has no
mapping layer at all and a column not listed in the schema cannot be selected
by any client. See the README section "The database row *is* the GraphQL type".

**Cost:** sqlc names a row type per query, so two queries returning the same
columns are two Go types; `internal/graph/rows.go` converts between them. A
query whose table is only known at run time cannot be generated; the one
example, the per-table row count, is hand-written in
`internal/db/tablestats.go`. Dynamic filters have to be written as SQL that
tolerates absent parameters, or as separate queries.

### SQLite in WAL with separate pools

One file, three pools (`internal/db/setup.go`, `Setup`): reads with up to 25
connections and `query_only`, application writes with exactly one, and the
queue with its own one. Every pragma is in the DSN (`internal/db/setup.go`, `open`) so it
applies to every connection the driver opens, including `busy_timeout`,
`foreign_keys` and WAL. Writers use `_txlock=immediate` (`internal/db/setup.go`, `open`), so a
transaction takes the write lock at `BEGIN` and waits for it instead of failing
on upgrade. The server migrates itself on boot from embedded migrations
(`internal/db/setup.go`, `Migrate`) through a cgo-free dbmate driver (`internal/dbmate/sqlite`), so
the binary is static.

**Cost:** one writer at a time, for the whole process. A write transaction that
waits on a network call holds every other writer, the job queue included, for
that long. The queue's separate pool keeps its polling from queuing behind the
application *for a connection*, but the database lock is still shared. One
process owns the file, so there is no second replica to scale out to. Backing
up a live WAL database needs SQLite's backup API or `VACUUM INTO`; copying the
file is not a backup.

### Integer timestamps and `dbtime.Stamp`

Every time column is `integer` unix seconds in a `STRICT` table. A sqlc
wildcard override maps every one of them to `dbtime.Stamp` (`sqlc.yaml`), which
embeds `time.Time` and implements `Scan` and `Value`
(`internal/dbtime/stamp.go`). GraphQL has one `Timestamp` object type bound to
`Stamp`, whose `unix` and `formatted(layout)` fields need no resolver. The
application writes every timestamp itself from `app.Now()` (`cmd/server/app.go`, `app.Now`), so a
case that stores a time declares `Now()` on its `Env` and its test fixes the
clock. The measurements and the two traps are in [rules.md](rules.md);
`list_queries.sh` checks that every `_at`/`_date` column is `integer`, has an
override, and sits in a STRICT table.

**Cost:** second precision. Two events in the same second have the same
timestamp, which is why the audit log pages by `id` rather than by
`created_at`. A table that needs sub-second ordering needs another column type,
and the wildcard override then needs another name. `Stamp` is always UTC.

### STRICT tables

`STRICT` makes SQLite enforce declared column types instead of storing whatever
it is given (`db/migrations/*.sql`). It is what keeps the timestamp override
honest: a text value cannot land in an integer column.

**Cost:** STRICT allows only `int`, `integer`, `real`, `text`, `blob` and
`any`. Booleans are `integer` 0/1, which is why `isAdmin` needs a resolver
(`internal/graph/schema.resolvers.go`, `adminUserResolver.IsAdmin`). The goqite table is STRICT too, but its
timestamps are `text` columns named `created`, `updated` and `timeout`, outside
the `_at` convention, so no override claims them
(`db/migrations/20260810090400_create_goqite.sql`).

### goqite instead of a broker

The queue is a table in the application database, so an enqueue is part of the
transaction that caused it (`cmd/server/jobs.go`, `app.EnqueueJob`). A ban that rolls back sends nothing;
a ban that commits cannot lose its notice. `cmd/server/tx_test.go` tests both.
There is no second system to deploy, monitor or back up.

**Cost:** the queue shares the single writer with the application. It is not a
broker: no fan-out, no consumer groups, no cross-process delivery beyond what
one SQLite file allows. goqite panics on a message whose job name nothing
registered, outside its own recover, and that would end the process; so before
the runner starts, `deadletterjobs` (`internal/case/deadletterjobs/resolve.go`, `Resolve`) moves such messages to the `jobs-dead`
queue, logs each at Warn and counts them in `starterkit_jobs_dead`. A job is
still best retired by draining its messages first.

### Cron as an enqueuer

See the job path above. **Cost:** a schedule is static, read from
configuration at boot (`cmd/server/cron.go`, `app.registerCronJobs`); schedules that users edit at run time
need their own table and a scheduler that reads it.

### Metrics and the shutdown order

Counters are incremented by the code that caused the event, declared on that
case's `Env` like any other port (`internal/case/banuser/resolve.go`, `Env`). Gauges are read
from the database by a background loop (`internal/metrics/metrics.go`, `Metrics.Run`), so
they cannot drift. `/metrics` has its own loopback listener
(`cmd/server/main.go`, `run` and `metricsHandler`), so the scraper does not need to pass the gate.

On SIGINT or SIGTERM the process context is cancelled (`cmd/server/main.go`, `run`), which
stops the gauge loop and cron's enqueues. The job runner has its own context,
cancelled only after the HTTP drain, and `run` waits for it before closing the
database (`cmd/server/main.go`, `app.startJobRunner`). The shutdown goroutine
(`cmd/server/main.go`, `run` and `app.shutdown`) then marks `/readyz` as failing, sleeps
`ShutdownDelay` so a load balancer can deregister the instance, and gives
in-flight requests `ShutdownTimeout` to finish. `run` waits for that goroutine
before returning (`cmd/server/main.go`, `run`), so the deferred `conns.Close()` does not pull
the database out from under a request that is still running.

**Cost:** `ShutdownDelay` is zero by default (`internal/appconfig/config.go`),
which is right for a single local process and wrong behind a load balancer; set
it. See [metrics-and-shutdown.md](metrics-and-shutdown.md).

### GraphQL with thin resolvers

One endpoint, schema first (`internal/graph/schema.graphqls`), generated by
gqlgen. Resolvers translate and delegate; the longest is about a dozen lines
(`internal/graph/schema.resolvers.go`). Logic lives in cases, where a unit test reaches it. A
list field returns a connection with `nodes` and `totalCount` as separate
resolvers, so a client asking for one does not pay for the other. A mutation
takes one `input` and returns a union with `ErrorPayload`.

**Cost:** a resolver per field that is not a plain column, a command per
mutation, a case per decision, and a generate step between the schema and the
code. The admin namespace means every real mutation is one level deeper than a
client might expect.

### `ErrorPayload` versus `error`

A failure the domain expects — invalid input, a user who does not exist, the
last admin — is an `ErrorPayload` returned with a nil error, carried by the
mutation's union and rendered by the UI (`internal/case/banuser/resolve.go`, `Resolve`). Validation
failures carry a message per input field (`internal/graph/model/errors.go`).
A failure the caller cannot fix — the database is gone — is an `error`, logged
with a reference and presented as `internal error, ref …`.

**Cost:** nothing enforces the split; it is a convention each case keeps. And
because the transaction commits on a nil error, see the cost under the command
method above.

### `moq` mocks and how the tests are layered

Three layers, each answering a different question:

| Layer | Where | What it proves |
|---|---|---|
| Case unit tests | `internal/case/*/resolve_test.go` | the decisions: validation, refusals, what is written, what is enqueued, which counter moves. The `Env` is a generated `moq` mock (`//go:generate` above `Env` in `internal/case/banuser/resolve.go`); assertions are `require`; a stored row is a `cupaloy` snapshot |
| Integration tests against real SQLite | `cmd/server/tx_test.go`, package tests | the mechanisms: commit, rollback, the nested guard, an enqueue that rolls back with its transaction, a `Stamp` round trip |
| Behavioural end-to-end | `e2e/`, Playwright against a real stand | the system: the audit log and the metrics as the oracle, and a snapshot of both against a reviewed baseline. See [behavioural-testing.md](behavioural-testing.md) |

**Cost:** mocks prove the case's calls, not that the SQL behind them is right;
that is what the second and third layers are for. Resolvers and the HTTP
middleware have no unit tests of their own and are covered end to end.

### The $mol SPA bridge

Each screen sits in `assets/ui/<name>/` with one `.graphql` file per operation
beside it. `npm run codegen:ui` (`package.json`, run by `make generate`)
turns each into a typed `.graphql.ts` function, `$starterkit_<path>()`, that
posts the operation through `$starterkit_gql_request`
(`assets/ui/gql/gql.ts`). A query subscribes to a marker that every mutation
bumps, so lists revalidate after a write without screen code asking. The SPA
learns its endpoint from `/settings.js` (`cmd/server/http.go`, `settingsScript`) rather than compiling a
path in. The built bundle is embedded with `go:embed` and served behind
`requireIdentity` (`assets/embed.go`, `cmd/server/http.go`, `app.routes`). See
[frontend-mol.md](frontend-mol.md).

**Cost:** $mol builds inside a separate mam workspace (`Makefile`, targets `ui`
and `ui-link`), which is an unusual toolchain to learn, and the codegen step has
to run after every schema change.

## How to add things

### A use case

1. `internal/case/<verb><noun>/resolve.go`: a narrow `Env`, `Input` and
   `Payload` (usually aliases of the generated GraphQL types), and
   `Resolve(ctx, env, input) (Payload, error)`. Add the `//go:generate` moq line.
2. `internal/graph/schema.graphqls`: the field under `AdminMutation` or
   `AdminQuery`, taking `input: XInput!` and returning `XOrErrorPayload!`. Run
   `make gqlgen`.
3. `internal/graph/schema.resolvers.go`: fill the generated stub with one line,
   `return r.env(ctx).X(ctx, input)`.
4. `internal/graph/resolver.go`: add `X(ctx, input) (x.Payload, error)` to
   `graph.Env`.
5. `cmd/server/commands.go`: the command, wrapped in `runInTx` if it writes.
6. `make moq`, then `resolve_test.go` next to the case.

If it does not compile, `app` is missing a method the case's `Env` asked for.
That is the wiring check.

If the resolver would only forward to one sqlc method, skip the case: put the
query on `graph.Env` and call it from the resolver.

### A query

1. Write it in `queries.read.sql` (`DBGet…`/`DBCount…` for `:one`, `DBList…`
   for `:many`) or `queries.write.sql` (`DB…`). Lowercase keywords, ASCII
   comments.
2. `make sqlc` — never bare `sqlc generate`.
3. Declare the method on the `Env` of the case that uses it. `app` already has
   it, promoted from the embedded `*db.Queries` or `*db.WriteQueries`.
4. `make lint` runs `internal/db/list_queries.sh`.

### A table

1. `make db-new name=<name>`, write `-- migrate:up` and `-- migrate:down`.
   Every table `strict`, every time column `integer`.
2. A new time column name needs its own `overrides` entry in `sqlc.yaml`.
3. `make db-up` regenerates `db/schema.sql`; then `make sqlc`.

### A job

1. A case package for the work, with an `Input` struct that has JSON tags.
2. `cmd/server/jobs.go`: a job name constant, a `jobs.Register` call in
   `registerJobs`, wrapped in `a.WithTransaction` if the job writes.
3. If a case enqueues it, a one-line port on `app`, like
   `EnqueueUserBannedNotice` (`cmd/server/jobs.go`, `app.EnqueueUserBannedNotice`), declared on that case's `Env`.
4. Make the handler safe to run twice.

### A cron entry

1. The job, as above.
2. A spec field in `internal/appconfig` (see below), like `AuditPurgeSpec`.
3. A `cronRunner.Add` entry in `cmd/server/cron.go` whose `Run` only enqueues.

### A metric

1. A field on `metrics.Metrics`, built in `metrics.New` and registered
   (`internal/metrics/metrics.go`, `New`).
2. A method `MetricsIncrease<Event>(ctx, …)` on `*Metrics`. `app` embeds
   `*metrics.Metrics`, so it is promoted.
3. Declare it on the `Env` of the case that causes the event, and assert it in
   that case's test.

A number that is a fact about the database rather than an event is a gauge set
in `Metrics.Update`, not a counter.

### A configuration value

1. A field on `appconfig.Config` (`internal/appconfig/config.go`), registered
   in `Get` as a flag whose default reads a `STARTERKIT_*` variable. Give it a
   named type when a bare string would need explaining, like
   `notifier.Address`.
2. A check in `Config.validate` if a value can be wrong in a way boot should
   refuse.
3. An accessor method on `app` (`app.go`, like `AuditRetention`), declared on
   the `Env` of the case that reads it.
4. A line in `.env.example`.

### An external service client

1. `internal/<service>/`: its own `Env`, a constructor taking it, typed
   parameters.
2. Embed it in `appState` and assign it in `newApp` after `a` exists, as
   `notifier.New(a, …)` is (`cmd/server/main.go`, `newApp`).
3. Cases declare the promoted methods they call on their own `Env`.

## What not to do

- **Do not open a transaction in a resolver or a case.** The command method
  owns it.
- **Do not call a third party inside a write transaction.** It holds the only
  writer for as long as the call takes.
- **Do not call one command from another,** or `WithTransaction` from inside
  one. It returns `ErrNestedTransaction`.
- **Do not type-assert `req.Env` or any `Env` to another case's contract** in
  the middle of logic. Declare a port.
- **Do not put state that must be shared on `app` itself.** Put it on
  `appState`; `app` is copied per transaction.
- **Do not run bare `sqlc generate`,** edit `db/schema.sql`, or put a select in
  the write file.
- **Do not add a `datetime` or `text` time column,** or a time column without
  its override.
- **Do not set pragmas with `db.Exec`.** They belong in the DSN.
- **Do not return an `error` for a domain failure,** or an `ErrorPayload` for an
  infrastructure one.
- **Do not let a cron entry do the work.** It enqueues.
- **Do not remove a job registration while its messages may still be queued.**
- **Do not expose `/metrics` on the public listener.**
- **Do not add a mapping struct between a row and a GraphQL type** unless the
  field genuinely differs; bind with `@goModel`.
