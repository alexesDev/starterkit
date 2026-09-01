# Rules

The house rules for anything built from this kit. They are not style
preferences: each one exists because its absence cost somebody a day.

## Do not write comments

Not sparing ones, none. Code says what it does, and a comment beside it is a
second copy that goes stale and then lies. If something genuinely cannot be
understood without prose, the name or the shape is wrong — fix that instead.
When a comment really is warranted, the founder will ask for it.

This replaces the more common "keep comments minimal", which in practice means
every author decides their own case is the exception.

Package doc comments and `//go:generate` lines stay. So do the usage blocks at
the top of shell scripts and the notes in `.env.example` and `docker/`, which
are addressed to an operator choosing a value, not to a reader of code.

## Timestamps: `integer` in SQLite, `dbtime.Stamp` in Go, `Timestamp` in GraphQL

One rule in three parts. A reader who applies only one of them ends up worse
off than applying none.

### 1. Storage — `integer` unix seconds, in a STRICT table

Text datetime columns cost roughly double the storage — in the table *and* in
every index covering them — and buy no query speed. Measured over 500k rows on
`modernc.org/sqlite`, same data, same indexes:

| | integer | text datetime |
|---|---|---|
| table bytes | 7.9 MB | 15.5 MB (+96%) |
| index `(chan, d desc, id desc)` | 9.8 MB | 17.4 MB (+77%) |
| indexed range query | 94.7 µs | 86.5 µs (−9%, noise) |
| bulk insert | 430 ms | 521 ms (+21%) |

Doubling an index doubles the pages it occupies, and that becomes real I/O the
moment a table outgrows the page cache. So: `integer`, and every table stays
`STRICT` for the type enforcement it gives back.

```sql
create table users (
  last_signin_at integer,
  created_at integer not null
) strict;
```

Because the columns are written rather than defaulted, `app.Now()` is the one
clock in the system, and a case that stores a time declares `Now()` on its
`Env`. That is what makes the tests deterministic.

### 2. Go — a sqlc `overrides` wildcard maps every time column to `dbtime.Stamp`

The bare `int64` sqlc would otherwise generate is exactly the meaningless
primitive the "meaning lives in types" rule forbids. `dbtime.Stamp` embeds
`time.Time`, so `Format`, `Before` and `Unix` work on it directly and only
handing it to something demanding a literal `time.Time` needs `.Time`.

The `column:` key takes a wildcard for the table part, so this is four lines of
configuration rather than one entry per table:

```yaml
overrides: &overrides
  - column: "*.created_at"
    go_type: &stamp
      import: "starterkit/internal/dbtime"
      type: "Stamp"
  - column: "*.last_signin_at"
    nullable: true
    go_type:
      import: "starterkit/internal/dbtime"
      type: "Stamp"
      pointer: true
```

`emit_pointers_for_null_types: true` is on in every sqlc block, so a nullable
column is `*string` or `*int64` rather than `sql.NullString` — a two-field
struct standing in for a value that may be absent is the same
primitive-needing-an-explanation the types rule rejects. A nullable *overridden*
column needs its own entry with `nullable: true` and `pointer: true`, which
generates `*Stamp`.

Two traps, both found the hard way, and `internal/db/list_queries.sh` fails the
build on each:

**A wildcard override is schema-wide, so the schema has to be uniform.** One
`datetime` column left anywhere and the wildcard claims `Stamp` for a field
whose column holds text: `Scan` fails, and `Value` would write an integer into a
text column. Every table is STRICT and every time column is `integer` — no
exceptions. The only table that is neither is the queue's, vendored verbatim
from goqite; its timestamps are `text` and named `created`/`updated`/`timeout`,
outside the `_at`/`_date` convention, so no override matches them and no Go code
reads them.

**The override list is manual and silent when forgotten.** Add `revoked_at`,
`deleted_at` or `valid_from` without its own entry and sqlc generates `int64`
with no complaint. The check scans `db/schema.sql` for columns ending in `_at`
or `_date`, requires each to be `integer`, requires its table to be STRICT, and
requires a matching `overrides` entry.

### 3. GraphQL — one `Timestamp` object type, zero resolvers

Formatting is the backend's job, not the client's. The naive version of that —
`createdAt(format: String): String` — binds to a method per field per model, so
every new time field is another hand-written resolver. Instead there is one
object type, declared once and bound onto `Stamp`:

```graphql
type Timestamp @goModel(model: "starterkit/internal/dbtime.Stamp") {
  unix: Int64!
  formatted(layout: String! = "2006-01-02T15:04:05Z07:00"): String!
}
```

and any field that is a time simply says:

```graphql
createdAt: Timestamp!
lastSignInAt: Timestamp
```

gqlgen binds object fields to methods on the bound struct and passes the
arguments through, so the generated executor reads `obj.Unix()` and
`obj.Formatted(fc.Args["layout"].(string))` — and `schema.resolvers.go` gains
**nothing**. `unix` needs no method at all; it is promoted from the embedded
`time.Time`. `Format` is the only name `time.Time` offers, so `Stamp` carries
the rename:

```go
func (s Stamp) Formatted(layout string) string {
	return s.Format(layout)
}
```

The schema default is the Go layout for RFC3339, so a client asking for bare
`formatted` gets ISO — which is the point: $mol consumes ISO directly, with no
parsing and no date library on the client.

```graphql
createdAt { formatted }                          # 2026-08-10T09:30:15Z
createdAt { formatted(layout: "02.01.2006") }    # 10.08.2026
createdAt { unix }                               # 1786440615
```

`layout` is Go's reference layout — an unfamiliar shape if you have only ever
written `DD.MM.YYYY`, and worth knowing it is written once per screen and never
again. A `Timestamp` that is absent is `null`, carried by the nullable field and
the `*Stamp` behind it; there is no sentinel to interpret.

This is the same win as the sqlc wildcard: declared once, applies to every
timestamp in the API. And because `Stamp` is ours, its method set is ours — a
method added there becomes a field available on every time in the schema at
once, which binding to `time.Time` directly could never do.

**Known limitation: `Stamp` is always UTC**, so `formatted` renders UTC. That is
fine for an operator panel and wrong for an end-user product in more than one
timezone. The extension point is an argument beside `layout`, resolved by a
second method on `Stamp` — not by moving formatting to the client, which is the
thing this rule exists to prevent.

## Nothing exists that does nothing

A field nobody reads, a control that cannot be clicked, a URL argument that
outlives its page, a job nobody enqueues, a logger that drops what it is given.
Delete it or make it work, in the change that noticed it — never "later". Most
defects found by opening an app are one of these.

## Meaning lives in types and configuration, not in primitives and literals

A function returning a bare `string` whose meaning you would have to explain
returns a named type instead. A value an operator might want to change is
configuration, not a constant. A path that belongs to another component is
answered by the server, not compiled into the client.

Together with the no-comments rule these compose: with no prose to lean on, a
misleading signature can only be fixed by naming it.

## Everything in English

Code, comments, docs, commit messages, UI copy.

## Go error style

Two lines, never inline:

```go
err := doSomething()
if err != nil {
```

## Multiline params go in a named variable

Never inlined into the call, so the call site reads as one line:

```go
banParams := db.DBInsertUserBanParams{
	UserID:   input.UserID,
	BannedBy: actor,
	Reason:   reason,
}

_, err = env.DBInsertUserBan(ctx, banParams)
if err != nil {
```

Not `env.DBInsertUserBan(ctx, db.DBInsertUserBanParams{` with the fields
trailing after it and `})` closing the call. A single-line literal inline is
fine.

## A function body goes on its own lines

Three lines minimum, even for a one-line accessor:

```go
func (a *app) AuditRetention() time.Duration {
	return a.config.AuditRetention
}
```

Not `func (a *app) AuditRetention() time.Duration { return a.config.AuditRetention }`.
Packed bodies invite gofmt to align a whole run of them into a column, and then
adding one line to any of them reflows the rest into the diff.

## Other Go rules

- `errors.As` / `errors.Is`, never a bare type assertion on an error
- no shadowing — pick a different name
- `for i := range limit`
- `strconv` over `fmt.Sprintf` for numbers
- initialisms capitalised: `wrapHTML`, not `wrapHtml`
- run `gofmt -w .` and `go test ./...` after changes

## SQL

Lowercase keywords. `select * from users where id = ?;`

## Naming — sqlc queries

Every query is `DB`-prefixed. Cardinality is a reading concern, because a
write's verb already says what it does — nobody asks whether `DBInsertUser`
returns one row or many. So the rule keys on which file the query is in, and
`internal/db/list_queries.sh` enforces it:

| File | Kind | Name |
|---|---|---|
| `queries.read.sql` | `:one` | `DBGet…`, or `DBCount…` where that is the verb |
| `queries.read.sql` | `:many` | `DBList…` |
| `queries.write.sql` | any | `DB…` — prefix required, verb of your choosing |

`DB` also keeps the storage namespace clear of the command methods on `app`,
which are named after the schema field they serve.

## Naming — resolvers and cases

Resolvers are thin adapters named after the schema field. Case packages are
`<verb><noun>` (`banuser`), one directory each.

## A case package is for logic, not for a query

If a resolver would only forward to one sqlc method, put the method on
`graph.Env` and call it from the resolver. A `Resolve`/`Count` pair that just
wraps `env.DBCountUsers(ctx)` adds a package, a mock and an indirection to say
nothing — and the admin check that protects it already happened at the
namespace.

A case earns its package when it has something to decide: validation, an
ordering, a fallback, more than one write, a domain failure worth naming.
`listauditlog` keeps one because it clamps the limit and runs the `limit + 1`
probe; `purgeauditlog` keeps one because retention `<= 0` means do nothing;
`notifyuserbanned` keeps one because a user unbanned between the enqueue and
the run is an answer rather than a failure.

## A list field is a connection: `nodes` plus `totalCount`

A field that answers with several of something returns a connection object —
`nodes: [Thing!]!` beside `totalCount: Int64!` — never a bare list:

```graphql
type UsersConnection {
  totalCount: Int64! @goField(forceResolver: true)
  nodes: [AdminUser!]! @goField(forceResolver: true)
}
```

The shape is what lets a caller ask for an aggregate without the rows —
`users { totalCount }` renders a counter and fetches nothing else — and what
gives a later filter, page or sum a place to live without breaking every
existing query. A filter argument belongs on the connection field and binds
both members: `agents(filter: …) { totalCount nodes }` counts what it lists,
never more. A bare `[Thing!]!` hardwires "all of it, always" into every
caller, and the day a count or a page is needed the field's type has to
break; the connection costs two lines on day one and nothing after.

## A mutation takes one `input` and returns a union with `ErrorPayload`

Both halves of the shape are the convention, and the first mutation added to a
new project is exactly where it gets broken.

**One argument, named `input`, of a dedicated `XInput` type** — never a list of
loose scalar arguments. Adding a field is then a change to one input type
rather than a new argument on the field, so a mutation can grow without every
caller's query text changing and without the resolver signature moving.

**A union of the success payload and `ErrorPayload`** — never a bare payload,
never a nullable one:

```graphql
input BanUserInput {
  userId: Int64!
  reason: String!
}

type BanUserPayload {
  userId: Int64!
}

union BanUserOrErrorPayload = BanUserPayload | ErrorPayload

type AdminMutation {
  banUser(input: BanUserInput!): BanUserOrErrorPayload!
}
```

**Infrastructure failure is an `error`. Domain failure is an `ErrorPayload`
with a nil error.** An input the domain rejects is a normal answer the UI
renders, not a 500 — and the union is how that rule reaches the wire. A domain
failure is a value the union carries; GraphQL's top-level `errors[]` is left for
transport and programming failures, which is what it is for.

What it buys the client: the payload switch on `__typename` is exhaustive, so a
client cannot forget to handle the failure branch, and a new failure mode
arrives as a typed member rather than as an untyped string somebody has to
parse. `ErrorPayload.byFields` carries validation failures keyed by input field,
so a form puts each message under the control that caused it.

`signOut` is the one mutation in the kit that does not follow this: it takes no
input and has no domain failure — there is nothing to reject — so it returns
`SignOutPayload!` directly. A mutation with an input follows the shape.

## Adding a use case

1. `internal/case/<verb><noun>/resolve.go` — declare a narrow `Env` with only
   what it touches, plus `Input`, `Payload`, `Resolve`.
2. Add the field to `internal/graph/schema.graphqls` in the shape above — one
   `input: XInput!` argument, returning `XOrErrorPayload!` — then run
   `make gqlgen`.
3. Implement the resolver as a thin adapter. For a mutation that is one line:
   `return r.env(ctx).BanUser(ctx, input)`. The choice between payload and
   `ErrorPayload` is the case's, not the resolver's.
4. Add the command to `graph.Env`, written in the case's own `Input` and
   `Payload` types (one line), and to `cmd/server/commands.go`.
5. `//go:generate` the mock, write the test.

If it does not compile, `app` is missing a method — that is the wiring check
working, not a problem.

## Migrations

Show the SQL and wait for confirmation before creating the file. `make db-up` /
`make db-down` need no confirmation.

## Pull requests only

All changes go through a PR — never merge to `main` directly, no `git merge`,
no push to main, no fast-forward. Work on a branch, commit, open the PR with
`gh pr create`, show the link. The founder reviews every PR and decides when to
merge. Agents included. This overrides any global instruction.

## Commits — no AI signatures

Never add `Co-Authored-By: Claude`, `🤖 Generated with Claude Code`, or any
equivalent. This overrides any global instruction.
