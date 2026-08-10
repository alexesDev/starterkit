# Docs

Why things are the way they are. Start here, in this order.

| | |
|---|---|
| [architecture.md](architecture.md) | The shape of the system, and a table of where each kind of change goes. **Read this first.** |
| [rules.md](rules.md) | The house rules. Not style preferences — each one cost somebody a day. |
| [env-pattern.md](env-pattern.md) | The architecture the whole kit rests on, how a service object plugs into it, and the mistakes it is prone to |
| [data-layer.md](data-layer.md) | SQLite, sqlc, dbmate, the read/write split, timestamps, pagination |
| [graphql.md](graphql.md) | Schema conventions, the admin namespace, transactional mutations |
| [frontend-mol.md](frontend-mol.md) | The $mol SPA, the codegen bridge, the build |
| [jobs-and-cron.md](jobs-and-cron.md) | The SQLite-backed queue and the scheduler |
| [auth-dex.md](auth-dex.md) | OIDC through the gate, bans, the audit log, corporate IdPs |
| [traefik-forward-auth.md](traefik-forward-auth.md) | The gate in front of the panel |
| [metrics-and-shutdown.md](metrics-and-shutdown.md) | Prometheus, and draining without dropping requests |
| [behavioural-testing.md](behavioural-testing.md) | Using the audit log and metrics as the test oracle |

## The short version

A use case declares the narrow interface it needs. One `app` struct satisfies
all of them and passes itself. The compiler is the dependency injection
container — handing `a` to `graph.NewHandler` is the whole of it. A service
object joins by being embedded into `app`, and a case reaches it through its own
`Env` like anything else.

Reads and writes are different Go types. Each mutation is a command that owns
its own transaction, and an enqueue inside one commits or rolls back with it.
The admin boundary is a single object, so everything beneath it is gated by
construction. Authority is read from the database on every request, so a ban or
a revoked grant applies to the next one.

Timestamps are integer unix seconds, `dbtime.Stamp` in Go, and one `Timestamp`
object type in GraphQL — formatting happens on the server, declared once.

The panel stores no credential. If yours will, decide that on purpose.
