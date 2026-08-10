# starterkit

A small, complete, working Go admin panel to start a project from. It boots,
serves a GraphQL admin panel behind an OIDC gate, has users, admins, bans and an
audit log, runs background jobs, exposes Prometheus metrics, drains cleanly, and
ships a $mol SPA with a typed codegen bridge to the schema.

It is not a framework and not a generator. It is a repository you copy and start
deleting from.

The rules it is written to are in **[docs/rules.md](docs/rules.md)**. Read them
before the first commit; the code is the first example of them anyone reads.

## The stack

Go · [gqlgen](https://gqlgen.com) · [sqlc](https://sqlc.dev) ·
[dbmate](https://github.com/amacneil/dbmate) · SQLite (WAL) ·
[$mol](https://mol.hyoo.ru) · Traefik + [Dex](https://dexidp.io) +
oauth2-proxy on docker compose · Prometheus ·
[goqite](https://github.com/maragudk/goqite) · Playwright

No ORM, no DI container, no broker, no Redis, no session store.

## Starting a project from it

```bash
git clone <this> myproject && cd myproject
rm -rf .git && git init
```

Then rename. `starterkit` appears as the Go module path, the `STARTERKIT_` env
prefix, the `$starterkit_*` SPA namespace, the compose project names, the base
path `/_system/starterkit` and the metric namespace:

```bash
grep -rIl 'starterkit\|STARTERKIT_\|Starterkit' . --exclude-dir=.git \
  | xargs sed -i 's/STARTERKIT_/MYPROJECT_/g; s/Starterkit/Myproject/g; s/starterkit/myproject/g'
go mod tidy && make generate && make test
```

Every package name under `internal/` is already neutral, so nothing moves.

Check `docker/`, `scripts/stack.sh` and `.env.example` afterwards: the loopback
addresses (`127.0.0.81`, `127.0.0.82`) and ports are chosen so this kit's stands
do not collide with anything else on the machine. If you run more than one
project from this kit at once, give each its own pair.

Then:

```bash
cp .env.example .env       # set STARTERKIT_BOOTSTRAP_ADMIN to your email
make db-up
make generate
make test
```

### What to delete first

Everything below is a worked example, not scaffolding you have to keep:

| Delete | If you do not want |
|---|---|
| `internal/notifier`, `internal/case/notifyuserbanned`, the `notify-user-banned` job | the service-object example (keep it as a template for a real client) |
| `internal/case/{banuser,unbanuser}`, `user_bans`, the ban UI | bans |
| `internal/case/{makeadmin,removeadmin}` | admin management from the UI |
| `assets/ui/auditlog`, `internal/case/listauditlog` | the audit log *screen* — keep `audit_log` itself, the boundary writes to it |
| `assets/ui/**` and `assets/embed.go` | the SPA at all — the GraphQL endpoint stands alone |
| `e2e/`, `playwright.config.ts`, `scripts/e2e.sh` | the behavioural suite |

What is not really optional: `internal/db`, `internal/dbtime`,
`internal/gateauth`, `internal/appreq`, `internal/audit`, `cmd/server/tx.go` and
the `app` struct. Those are the architecture.

## The database row *is* the GraphQL type

This is the property that removes the most code, and it is not obvious from
reading either side on its own.

sqlc generates a Go struct per query result. A GraphQL object binds straight
onto one with `@goModel` — there is no DTO in between restating the same fields,
and no mapping function to keep in step:

```graphql
type User @goModel(model: "starterkit/internal/db.User") {
  id: Int64!
  email: String!
  name: String!
  createdAt: Timestamp!
}
```

The GraphQL type is then a **view** over the row rather than a dump of it. It
chooses which columns exist at the API and under what names: `db.User` also has
`OidcIssuer` and `OidcSubject`, and because they are not listed above they
cannot be selected by any client. Exposure is opt-in per field, which is the
opposite default from `json.Marshal` on a table row.

Where the row and the API need to diverge, a resolver is the seam — per field,
not per type. Three kinds of divergence, all of them real in this kit:

- **Computed.** `AdminUser.isAdmin` is `Boolean!` in the schema, but the column
  behind it is SQLite's `0`/`1`. `@goField(forceResolver: true)` and a one-line
  resolver in `internal/graph/schema.resolvers.go` do the conversion:
  `return obj.IsAdmin != 0, nil`.
- **Deferred to another query.** `UsersConnection.totalCount` and `nodes` are
  both forced resolvers over an empty marker struct, so a client asking for
  `nodes` alone never pays for the `count(*)`.
- **Given behaviour.** Every time column binds to `dbtime.Stamp`, and the
  `Timestamp` object type resolves its fields to methods on it, so `formatted`
  and `unix` exist on every timestamp in the API without a resolver anywhere.

So the ninety per cent of fields that are just the column get no translation
layer at all, and the ten per cent that need one pay for it individually. Adding
a mapping struct "for consistency" would make the cheap case as expensive as the
expensive one.

## The linter is deliberately aggressive

`make lint` failing on your first commit is expected. `.golangci.yaml` is 210
lines enabling **62 linters**, and it is inherited on purpose rather than tuned
down to what a new repository happens to pass.

Beyond `go vet` it enforces, among others: complexity ceilings (`cyclop` at 30
with a package average of 15, `gocognit` 30, `funlen` 150 lines / 75
statements, `nestif`), `gochecknoglobals` and `gochecknoinits`, `dupl`,
`exhaustive` on switches over enums, `errorlint` for `errors.Is`/`As`,
`goconst`, `godot` (a doc comment ends in a period), `gosec`, `depguard`
(`log`, bare `math/rand` and a few packages are denied outright), plus the
`goimports` local prefix and `golines` wrapping at 160 columns.

`nolintlint` is on with `require-explanation` and `require-specific`, so an
escape hatch has to name its linter and say why. The whole kit needs exactly
two:

```go
//nolint:gochecknoglobals // -X can only target a package-level var.
func init() { //nolint:gochecknoinits // driver registration
```

Generated code is excluded rather than silenced case by case:
`internal/db/{db,models,queries.*}.go`, `internal/graph/generated/`,
`internal/graph/model/models_gen.go`, `internal/graph/schema.resolvers.go`,
every `mocks_test.go`, and `assets/ui/**/-/`. The adapted upstream driver in
`internal/dbmate/sqlite/` is excluded too, from the linters only.

`.golangci.yaml` carries that list **twice** — under `formatters.exclusions` and
again under `linters.exclusions` — so a new generated path has to be added in
both places or it will be formatted-but-not-linted, or the reverse. The two
copies already differ by one entry today, which is the failure mode in
miniature.

Tests are held to less: `_test.go` is exempt from `bodyclose`, `dupl`,
`errcheck`, `funlen`, `goconst`, `gosec` and `noctx`.

### Relaxing it

From writing this kit under the config, the ones a new project is most likely to
want gone first, in that order:

1. **`godot`** — noise unless you already end every comment in a period.
2. **`gochecknoglobals`** — the first package-level `regexp.MustCompile` or
   lookup table starts an argument you may not want to have.
3. **`dupl`** — one case package per use case means near-identical files by
   design; the kit is under the token threshold with five commands and a project
   with thirty will not be.
4. **`funlen` / `cyclop` / `gocognit`** — composition roots are the pressure
   point. `appconfig.Get` is already 60 lines and `run` is 86, both well inside
   the limits but both the kind of function that only grows.
5. **`depguard`** — its denylist is this project's, not yours.

Delete the line from the `linters.enable` list rather than blanket-`nolint`ing
at the call sites, and say in the commit why.

Two linters the parent project also runs are **not** enabled here: `nilnil` and
`wrapcheck`. Add them if you want it stricter still — `wrapcheck` in particular
will make you wrap every error crossing a package boundary, which this kit does
by hand already.

## Commands

```bash
make dev        # run the server with live reload, against a running stand
make generate   # sqlc -> gqlgen -> moq -> graphql-codegen, in that order
make test
make lint
make db-new name=x && make db-up
make up         # docker compose: traefik, dex, oauth2-proxy
make mol-dev    # mam dev server — in dev Traefik serves the SPA from it
make ui         # one-off SPA build, which is what the image embeds
scripts/e2e.sh  # the behavioural suite against a real stand
```

`make generate` order matters: sqlc reads `db/schema.sql`, which `make db-up`
regenerates. Run migrations first.

## Names

Both stands answer on their own loopback address, and every hostname has to
resolve there or discovery fails minutes later inside a container. One dnsmasq
line per zone:

```
address=/sk.localbox/127.0.0.81
address=/e2e.sk.localbox/127.0.0.82
```

`host/etc/dnsmasq.d/` and `host/etc/systemd/resolved.conf.d/` carry the files to
copy. After that, `http://panel.sk.localbox` is the dev panel and
`http://dex.sk.localbox/dex` is the issuer — the same string for the browser,
for oauth2-proxy and for the panel, which is the property the whole login flow
rests on.

## First run

```bash
make up          # the gate
make mol-dev     # the SPA, own terminal
make dev         # the panel
```

Open <http://panel.sk.localbox>, sign in as `admin@example.com` / `passpass`
(a committed development identity in `docker/dex/config.yaml` — delete it before
anything real), and the bootstrap rule promotes that account to admin because
`admins` is empty.

## Where to read next

[docs/architecture.md](docs/architecture.md) is the map, and
[docs/README.md](docs/README.md) is the index. The one idea worth internalising
before touching anything is in [docs/env-pattern.md](docs/env-pattern.md), which
is this kit's operational version of
[The Env Pattern: One IO Spine, Portable Business Logic](https://trip2g.com/en/thoughts/env_pattern)
— the essay the architecture comes from.

## Before you deploy anything built from this

- Delete `enablePasswordDB` and `staticPasswords` from `docker/dex/config.yaml`
  and point Dex at the real IdP.
- Override the committed development secrets in `docker/compose.yml`.
- Set `STARTERKIT_SHUTDOWN_DELAY` above your load balancer's health-check
  interval times its failure threshold.
- Give Dex durable storage; losing its signing keys signs everyone out at once.
- Run `scripts/refuse_system_paths.sh <origin>` against the deployed origin.

The reasoning for each is in [docs/auth-dex.md](docs/auth-dex.md),
[docs/traefik-forward-auth.md](docs/traefik-forward-auth.md) and
[docs/metrics-and-shutdown.md](docs/metrics-and-shutdown.md).
