# The Env pattern

The architecture this kit is built on. It has no established name; it is a
natural evolution of idiomatic Go applied to a real monolith, and it is what
seven years of one settled into.

The pattern is described in full, with the history and the audit that produced
the warnings near the bottom of this page, in
[The Env Pattern: One IO Spine, Portable Business Logic](https://trip2g.com/en/thoughts/env_pattern).
Read that for the argument. This page is the operational half — what the kit
actually does, and what to do on a Tuesday — and every claim on it was checked
against the code in this repository.

## The idea

Every use case lives in its own package under `internal/case/`. Each declares a
minimal `Env` interface — only the methods it actually needs:

```go
// internal/case/unbanuser/resolve.go
type Env interface {
	DBDeleteUserBan(ctx context.Context, userID int64) error
}

type (
	Input   = model.UnbanUserInput
	Payload = model.UnbanUserOrErrorPayload
)

func Resolve(ctx context.Context, env Env, input Input) (Payload, error)
```

The `Env` names exactly what this case touches. Someone reading it knows what
the case can reach without leaving the file.

A GraphQL resolver is a thin adapter:

```go
func (r *adminMutationResolver) UnbanUser(ctx context.Context, obj *model1.AdminMutation, input model.UnbanUserInput) (model.UnbanUserOrErrorPayload, error) {
	return r.env(ctx).UnbanUser(ctx, input)
}
```

`app` passes itself as `env`. The use case sees only its narrow slice.

## The app struct

`cmd/server/app.go` holds every connection, client and cache. It satisfies
every case `Env` at once because Go interfaces are structural — nothing
declares the relationship.

```go
type app struct {
	*appState

	*db.Queries
	*db.WriteQueries

	currentTx *sql.Tx
}
```

Most `Env` methods are free: `db.Queries` and `db.WriteQueries` are embedded, so
every sqlc-generated method is promoted onto `app` automatically. You only write
a method by hand when it is a real port — a client call, a cross-case call, or
something that needs a transaction.

**The two-level split is load-bearing, not style.** `app` is shallow-copied to
make a transaction-scoped env (`newEnv := *a`, in `withTxQueries`). Only the
fields that must change per transaction live on `app` itself. Everything
process-lifetime lives behind the `*appState` pointer and is therefore shared by
every copy. Put a mutex, a cache or an atomic directly on `app` and each
transaction silently gets its own copy of it — a bug that only shows up under
concurrency.

## Services plug in the same way

An external service client — a mailer, an object store, a third-party API —
declares its own `Env`, takes it at construction, and is **embedded into
`app`**. Its methods are then promoted, exactly like the sqlc ones, and a case
reaches them through its own narrow `Env` without knowing the service exists as
a type.

`internal/notifier` is the worked example. It declares what it needs:

```go
// internal/notifier/notifier.go
type Env interface {
	Logger() logger.Logger
	MetricsIncreaseNotification(ctx context.Context, kind string)
}

func New(env Env, from Address, origin Origin) *Notifier

func (n *Notifier) NotifyUserBanned(ctx context.Context, to Address, reason string) error
```

`appState` embeds it:

```go
type appState struct {
	*metrics.Metrics
	*notifier.Notifier
	// ...
}
```

and `newApp` constructs it against `a` itself, after `a` exists:

```go
a.Metrics = metrics.New(a, config.Metrics)
a.Notifier = notifier.New(a, config.NotifierFrom, notifier.Origin(config.PublicURL))
```

A case that wants to send a notice names the promoted method on its own `Env`
and nothing more:

```go
// internal/case/notifyuserbanned/resolve.go
type Env interface {
	DBGetUserByID(ctx context.Context, id int64) (db.User, error)
	DBGetUserBan(ctx context.Context, userID int64) (db.UserBan, error)
	NotifyUserBanned(ctx context.Context, to notifier.Address, reason string) error
}
```

Three properties fall out of that shape. The service is testable on its own,
because its `Env` is two methods. The case is testable without it, because
`moq` generates a `NotifyUserBanned` stub from the case's own contract. And
replacing the delivery — the kit's notifier writes to the log; a real one dials
SMTP — changes one file and no case, because the seam is the service, not the
call site.

The same trick handles the awkward moment in `newApp`: a service whose `Env` is
`app` cannot be built before `app` exists, so it is assigned onto `a`
afterwards. `metrics.Metrics` has the same shape and the same two lines.

The parameters are named types (`notifier.Address`, `notifier.Origin`) rather
than bare strings, because a constructor taking three strings is a constructor
whose arguments can be swapped silently — see [rules.md](rules.md).

## Wiring is a compile error, not a runtime one

```go
graph.NewHandler(a)
```

`NewHandler` takes a `graph.Env`, so that call in `cmd/server/http.go` is the
whole DI container. Add a method to a case `Env`, forget to implement it on
`app`, and the build fails naming the contract and the missing method. That is
not a claim; it is what `go build ./...` prints:

```
cmd/server/commands.go:35:35: cannot use env (variable of type *app) as
unbanuser.Env value in argument to unbanuser.Resolve: *app does not implement
unbanuser.Env (missing method NotifyUserUnbanned)
```

The proof must be a *side effect of the wiring*, not a separate discipline. A
file full of `var _ pkg.Env = app` is a manual ritual that drifts. Constructors
and registration sites do the same job and cannot go stale, because deleting
them deletes the feature:

```go
// cmd/server/jobs.go — jobs.Register captures a typed env in a closure
jobs.Register(a, jobNotifyUserBanned, 0, func(ctx context.Context, params notifyuserbanned.Input) error {
	return notifyuserbanned.Resolve(ctx, a, params)
})
```

The same principle rules out a fake generic. Job registration could have been
`Register[T, P]` with `T` the Env type and `env.(T)` inside — type safety that
does not exist, dressed as a type parameter. `jobs.Register` keeps only the
honest parameter, `P`, which does real work by unmarshalling the payload; the
env is captured in the closure and checked by the compiler. A generic earns its
place when the alternative is copying code. It does not earn its place when it
performs safety it cannot provide.

## The edge of the system: one fat interface, and it is honest

If every case declares a narrow `Env`, something has to gather them. Here that
is `graph.Env`, one line per case:

```go
type Env interface {
	audit.Writer

	BanUser(ctx context.Context, input banuser.Input) (banuser.Payload, error)
	UnbanUser(ctx context.Context, input unbanuser.Input) (unbanuser.Payload, error)
	// ...

	// Plain reads sit here directly.
	DBListUsers(ctx context.Context) ([]db.DBListUsersRow, error)
	DBCountUsers(ctx context.Context) (int64, error)
}
```

**One difference from the essay is worth knowing, because it is a consequence of
a decision made elsewhere in this kit.** The essay's `graph.Env` *embeds* the
case contracts — `hidenotes.Env`, `pushnotes.Env` — because there a resolver
calls `hidenotes.Resolve(ctx, r.env(ctx), input)` and therefore needs everything
the case needs. Here a resolver calls a *command* on `app`, because a command
owns its transaction ([graphql.md](graphql.md)), so what the edge needs is the
command signature, not the case's dependencies.

The line is still written in the case's own `Input` and `Payload` types, which
keeps the property that matters: the aggregate names the case packages, so it
reads as a table of contents of everything GraphQL serves, and a change to a
case's contract reaches this interface at compile time rather than silently. It
is a sum of contracts either way — just of the commands rather than of the
`Env`s.

A fat interface at the boundary is not a retreat from interface segregation; it
is its aggregate. Segregated contracts have to meet somewhere, and one explicit
place beats being smeared across the codebase.

The second thing it buys: the app-graph import cycle breaks without tricks. The
`graph` package cannot import `cmd/server`, so it declares the contract on its
side and `app` fulfils it. A consumer-defined interface, just a big one.

Two commands cannot share a name here, so keep them specific: the aggregate is
one flat interface and a second `Update` would collide with the first.

## Transactions: why Begin() does not return Env

The classic question. The naive answer, a `Begin() Env` method on the interface,
does not compile:

```go
type Env interface {
	Begin() Env
}

func (e *RealEnv) Begin() *RealEnv { ... }
// cannot use e (type *RealEnv) as type Env:
//   wrong type for Begin method: have Begin() *RealEnv, want Begin() Env
```

Go has no covariant return types. The answer is that transactions belong to the
`app` layer, not to the cases. `WithTransaction` passes the closure a concrete
`*app` — a copy with transaction-bound queries:

```go
func (a *app) WithTransaction(ctx context.Context, fn func(context.Context, *app) (bool, error)) error
```

Cases know nothing about transactions. The covariance problem is not solved; it
is sidestepped, because inside `app` you do not need the interface.

`cmd/server/commands.go` wraps that in `runInTx` for the common case, and
`cmd/server/tx_test.go` pins the behaviour, including the nested-transaction
guard: the write pool holds one connection, so a nested `BeginTx` would deadlock
against the transaction already holding it.

## A query is usually not a case

A case package earns its place when there is a decision in it: validation, a
permission rule beyond the admin boundary, several writes that must agree,
something to emit. `DBListUsers` has none of that, so it goes on `graph.Env` and
the resolver calls it. A `Resolve` that only forwards to one query and wraps the
error is a layer, not a use case — and it costs a package, a second `Env`, a
generated mock and a test that asserts the mock was called.

## Testing

Every `Env` is small, so the mock is small. `//go:generate` lines produce them:

```go
//go:generate go tool github.com/matryer/moq -out mocks_test.go -pkg unbanuser_test . Env
```

`make moq` regenerates. A test mocks three methods, not an application, and it
lives next to the case it covers. There is no God mock in this repository and
there should never be one.

Assertions are `github.com/stretchr/testify/require` — `require`, not
`assert`. `require` stops the test at the first failure, so the report names
the cause; `assert` carries on and buries it under its consequences.

Where a test would check many fields of one value,
`github.com/bradleyjkemp/cupaloy/v2` snapshots the value instead of a column
of `require.Equal` lines. `banuser` does this with the row it stores:

```go
cupaloy.SnapshotT(t, stored)
```

The diff runs against a committed file in the package's `.snapshots/`
directory, and a field nobody thought to assert still shows up the day it
changes. `UPDATE_SNAPSHOTS=true go test ./...` re-records; read the diff
before committing it, because the baseline is the assertion. Everything inside
the snapshot must be deterministic — `banuser` can snapshot a row with a
timestamp because the mock's `Now` is fixed.

Those three — moq, require, cupaloy — are the whole toolbox. A new test that
wants another library is a design question, not a dependency to add.

## What this pattern is prone to

Learned the hard way, over seven years and one audit of sixty-one cases. All
three families below grew from the same root: a place where the check was taken
away from the compiler and handed to the runtime.

**Runtime casts to another case's contract.** If a case needs another case's
work, do not reach for `req.Env.(othercase.Env)` mid-logic — the compiler cannot
see it, and a missing method becomes a silent no-op. In the audit this had
already happened five times, and the symptom was webhooks quietly not firing.
Two legal options:

- *A port.* The calling case declares the method on its own `Env`; `app`
  implements it in one line. `banuser` does exactly this with
  `EnqueueUserBannedNotice`, which is `a.EnqueueJob(...)` on the other side and
  says nothing about how the notice is delivered.
- *Embedding.* If a case genuinely needs another's whole contract, embed it —
  `type Env interface { othercase.Env; /* its own methods */ }`. Duplicate
  methods across embedded interfaces are legal since Go 1.14, which is what
  makes this scale.

Ports are for orchestration ("do X, do not tell me how"). Embedding is for
shared capability ("I need everything that layer can do"). There is no third
way.

**Embedding has no instance in this kit** — no case here needs another's whole
contract, and inventing one to illustrate the mechanism would be a package that
does nothing. The port does have one, and no case in this repository imports
another case.

**Business logic drifting into resolvers.** In the audit, five resolvers had
grown to 34–73 lines each, including access-control checks living in the
transport layer where case tests cannot reach them; extracting the logic put
them back at 3–13 lines. A mutation resolver in this kit is one line, and the
longest resolver of any kind is twelve. Anything longer — especially a
permission check — belongs in a case, where a test can reach it.

**Duplicated retry and timeout logic.** Four copies, all four with the same
three bugs: a timeout derived from `context.Background()` that ignores
shutdown, a recursive retry with a fresh budget per attempt and no limit, and
`time.Sleep` instead of cancellable waiting. The pattern is not to blame and it
did not save anyone either. Copied code copies bugs; only noticing does.

Where the wiring stayed static, nothing rotted in seven years.

## Places the pattern is deliberately not pressed all the way

**Two runtime casts, both at a boundary and both deliberate.**
`Resolver.env(ctx)` casts the request-scoped `req.Env` to `graph.Env`, and
`app.txEnvFromCtx` casts it to `*app` to find an open transaction. Removing them
would mean giving up context-carried env, and that is the same mechanism that
swaps the env inside a transaction. They stay, backed from the other side by
`graph.NewHandler(a)`, which the compiler checks. Neither is the anti-pattern
above: neither reaches for a *foreign case's* contract in the middle of business
logic.

**Transport machinery in the transport layer.** A subscription pump, a file
upload, a streaming response: that is plumbing, and it has no business in a
case. Extract the authorization out of it and test that; leave the pump.

**One-line cron delegations.** A scheduled job whose whole body is
`a.EnqueueJob(...)` does not get a `resolve.go`, an `Env` and a mock. That would
be ceremony for ceremony's sake.

## Where it comes from

The closest name in the Go community is **consumer-defined interfaces**: the use
case owns its contract, not the dependency. It combines interface segregation
(every case sees its slice of the world), dependency inversion (cases depend on
abstractions, not on `*app`), and hexagonal architecture (every `Env` is a port,
`app` is the adapter) — with Go interfaces as the container and no framework at
all.

- [The Env Pattern: One IO Spine, Portable Business Logic](https://trip2g.com/en/thoughts/env_pattern) — the source this kit implements
- Peter Bourgon, [Go for Industrial Programming](https://peter.bourgon.org/go-for-industrial-programming/)
- Dave Cheney, [SOLID Go Design](https://dave.cheney.net/2016/08/20/solid-go-design)
- [benbjohnson/wtf](https://github.com/benbjohnson/wtf) — the same family, inverted: the interface is declared by the provider, one per entity, and mocks are handwritten

The distinctive part here is that `app` passes *itself*: one object is the
adapter for every port at once, and the compiler verifies it across every use
case in the repository.
