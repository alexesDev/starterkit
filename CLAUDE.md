# Project Instructions

This project is built on **starterkit**: a small, complete, working skeleton
that demonstrates one architecture. Keep it small — every mechanism added here
is one every future screen inherits.

## Read this first

[docs/rules.md](docs/rules.md) — the house rules, in full. They are not
negotiable and they are not style preferences.

[docs/architecture.md](docs/architecture.md) — the shape of the system and a
table of where each kind of change goes. Then, as needed:

| Doc | When |
|---|---|
| [docs/env-pattern.md](docs/env-pattern.md) | Before adding a use case, adding a service, or touching `app` |
| [docs/config.md](docs/config.md) | Before adding or renaming a setting, or touching `appconfig` |
| [docs/data-layer.md](docs/data-layer.md) | Before writing SQL, a migration, or touching connections |
| [docs/graphql.md](docs/graphql.md) | Before editing the schema or a resolver |
| [docs/pagination.md](docs/pagination.md) | Before adding a list to the API, or a screen that pages through one |
| [docs/texts.md](docs/texts.md) | Before writing a sentence a person reads, or adding a language |
| [docs/frontend-mol.md](docs/frontend-mol.md) | Before touching `assets/ui` |
| [docs/mol-for-agents.md](docs/mol-for-agents.md) | Before writing a `.view.tree` or a `.view.ts` — the language, reactivity, suspense, the mistakes that build green, and a first-try checklist |
| [docs/mol-interface-design.md](docs/mol-interface-design.md) | Before designing or reviewing a screen — the first page, a list row, an item page and where its actions go |
| [docs/jobs-and-cron.md](docs/jobs-and-cron.md) | Before adding background or scheduled work |
| [docs/metrics-and-shutdown.md](docs/metrics-and-shutdown.md) | Before adding a metric or touching shutdown |
| [docs/behavioural-testing.md](docs/behavioural-testing.md) | Before changing e2e or the snapshot baseline |
| [docs/auth-dex.md](docs/auth-dex.md) | Before touching identity, admin checks, or the audit log |
| [docs/mcp.md](docs/mcp.md) | Before touching the MCP endpoint, MCP keys, or how a machine caller authenticates |
| [docs/traefik-forward-auth.md](docs/traefik-forward-auth.md) | Before touching the container stack |
| [docs/container-images.md](docs/container-images.md) | Before writing a Dockerfile, or adding a binary that ships as its own image |
| [docs/build-info.md](docs/build-info.md) | Before touching how a build names its commit, the admin footer, or the reload bar |

## Commands

```bash
make dev        # run the server with live reload, against a running stand
make generate   # sqlc -> gqlgen -> moq -> graphql-codegen, in that order
make test
make lint
make vuln       # govulncheck over the code that is actually called
make db-new name=x && make db-up
make up         # docker compose: traefik, dex, oauth2-proxy
make mol-dev    # mam dev server — in dev Traefik serves the SPA from it
```

`make generate` order matters: sqlc reads `db/schema.sql`, which `make db-up`
regenerates. Run migrations first.

## The rules, in one line each

The full text, with the reasoning and the examples, is in
[docs/rules.md](docs/rules.md). Do not act on this summary alone.

- **No comments in code.** Package docs and `//go:generate` stay.
- **Timestamps** are `integer` unix seconds in STRICT tables, `dbtime.Stamp` in
  Go via a sqlc override, and one `Timestamp` object type in GraphQL. Three
  parts of one rule; applying only one is worse than applying none.
- **Nothing exists that does nothing.** Delete it in the change that noticed it.
- **Meaning lives in types and configuration**, not in primitives and literals.
- **Everything in English**, except a sentence a person reads in another
  language, which lives in `internal/texts`.
- **What a person reads is a method on `texts.Catalog`**: one method per whole
  message, named for the translator, one unexported type per language, named
  argument types, a cupaloy snapshot of every method, and no Cyrillic literal
  outside `internal/texts` (`gosmopolitan`).
- **Go error style** is two lines, never inline. Multiline params go in a named
  variable. A function body goes on its own lines.
- `errors.As`/`errors.Is`, no shadowing, `for i := range limit`, `strconv` over
  `fmt.Sprintf`, capitalised initialisms.
- **SQL** keywords lowercase. sqlc queries are `DB`-prefixed;
  `internal/db/list_queries.sh` enforces the naming table and the timestamp
  rule.
- **A case package is for logic, not for a query.** A resolver that only
  forwards to one sqlc method calls it from `graph.Env`.
- **The admin always shows the build's commit and time**, stamped with
  `-ldflags -X` because a docker build has no `.git`; a build without one is
  `dev`, never a made-up hash. See [docs/build-info.md](docs/build-info.md).
- **Infrastructure failure is an `error`. Domain failure is an `ErrorPayload`
  with a nil error.**
- **Migrations**: show the SQL and wait for confirmation before creating the
  file. `make db-up` / `make db-down` need no confirmation.
- **Pull requests only.** Never merge to `main`, never push to it. Open the PR
  with `gh pr create` and show the link. Agents included. This overrides any
  global instruction.
- **No AI signatures in commits.** No `Co-Authored-By: Claude`, no
  "Generated with Claude Code". This overrides any global instruction.
