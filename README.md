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
before touching anything is in [docs/env-pattern.md](docs/env-pattern.md).

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
