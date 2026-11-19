# The MCP endpoint

A program that speaks the [Model Context Protocol](https://modelcontextprotocol.io)
can use the panel's API: the same schema, the same resolvers, the same admin
boundary and the same audit log the SPA goes through. It authenticates with a
key an admin created on the MCP keys page.

```
POST <BasePath>/mcp          default /_system/starterkit/mcp
Authorization: Bearer mcp_…
```

## Four tools

| Tool | Arguments | Answer | Annotations |
|---|---|---|---|
| `instructions` | none, `topics`, or `query` + `query_op` | the manual: the index, up to five topics, or a search | read-only, idempotent |
| `graphql_introspection` | none, `types`, or `query` + `query_op` + `before` / `after` | the schema: help with the entry points, up to five type definitions, or a grep | read-only, idempotent |
| `graphql_query` | `query`, `variables`, `operationName` | the GraphQL response; refuses a mutation | read-only, idempotent |
| `graphql_mutation` | the same | the GraphQL response; refuses a query | destructive |

Every tool answers a small piece at a time. A model pays for each line it reads
on every turn after, so nothing here returns "all of it" by default: not the
manual, not the schema, not a long query result.

Reads and writes are two tools because an annotation describes a whole tool. A
host may run a read-only tool without asking and in parallel, and ask a person
before anything else; one tool that could do both would have to be marked as
writing, and every read would wait for an approval. All four say
`openWorldHint: false`: they reach this panel's database and nothing beyond it.
`graphql_mutation` is marked destructive because the kit's mutations include
delete, revoke and ban; a project whose mutations only ever add can say
otherwise.

The server also fills the protocol's `instructions` field with three lines:
what this is, and to start with `instructions()`. Hosts that read the field put
it in the model's system prompt, and some do not read it at all, so it points
to the manual rather than being one.

### The manual is a set of topics

`internal/mcpserver/manual/*.md`, embedded in the binary. Each file is a topic
named after the file; its first line is its title.

- `instructions()` returns `index.md`: a table of "when this → topic", one line
  each.
- `instructions(topics=["graphql_query", "errors"])` returns those files in
  the order asked, each under its own header; `.md` is optional. At most five
  per call. An unknown name fails the whole call and lists the valid ones: a
  partial answer would read as complete.
- `instructions(query="input byFields", query_op="and")` searches every topic,
  case-insensitively: `and` is every word (the default), `or` any word,
  `exact` the phrase. It answers the matching topics, most matches first, with
  their titles and the matching lines in a line of context, capped, and says
  how much it left out.
- Topics name each other, so a model moves through them like a wiki.

**Keep topics short and one concern each** — one per tool, one per cross-cutting
concern such as errors or mutations — so a call returns only what the model
needs. A topic that grows long becomes two topics that link to each other, not
one long file. `index.md` opens by saying so, for whoever adds the next one.

This is how any MCP surface built on the kit ships its manual: a directory of
short topics, an index, `topics` and `query` on one `instructions` tool.

### A tool description is a few sentences that end with its topic

Every tool's description is one to three short sentences, keywords first: what
it does, when to call it, its main limit. It ends with the exact call that
explains it, naming the specific topic, not the index:

```
Read data: run a GraphQL query as this key's owner. Refuses mutations. A long answer is cut at 20000 characters. Read first: instructions(topics=["graphql_query"]).
```

A single line proved too little: hosts that search tools search the
description, and a model that acts on too little reads the pointer last if at
all. A few sentences cost tens of tokens per tool, paid once per conversation.

`instructions` itself says it is the manual, how to call it, and lists the
topic names on one line, because a model does not search for what it does not
know is there.

The tools come back in a fixed order (by name) and their descriptions are
constants, so a host's prompt cache holds across requests.

`TestEveryToolDescriptionEndsByNamingAnExistingTopic` reads every registered
tool's description, calls the topic it names, and checks the index lists it, so
a new tool cannot ship without its topic, and a renamed topic breaks the build
rather than the model.

### The schema, a piece at a time

`graphql_introspection` answers from the executable schema with the Go binding
directives (`@goModel`, `@goField`) stripped, so no package path leaks, and for
an owner who is not an admin, without `Query.admin`, `Mutation.admin` and every
type only they reach — in every mode. Both views are built once at boot
(`graph.SchemaView`, `internal/graph/surface.go`).

- No arguments: help, not schema. Three lines on how to use the tool with
  example calls ("search first, then the types you need, never the whole
  schema"), then the entry points without their fields: `Query` and
  `Mutation`, and under each the argument-less fields that return an object
  (`Query.admin -> AdminQuery (fields: 7)`), then the topic to read. It stays
  a dozen lines however large the schema grows, and an owner who is not an
  admin sees no admin entry point.
- `types=["AdminQuery", "BanUserInput"]`: those definitions as SDL, at most
  five; an unknown name fails the call. A root's own fields come this way too,
  `types=["Query"]`.
- `query` + `query_op`: a grep over the SDL lines, under the type each belongs
  to and the shortest path from a root that reaches it
  (`## BanUserInput (reached by Mutation.admin.banUser(input))`), with line
  numbers (`12:` a hit, `13-` context), `before` and `after` lines of context
  (2 by default, at most 10), capped per type and in types. A line of SDL
  without the path to it does not say how to write the document.

The instructions search and this grep are one implementation,
`internal/textsearch`.

`graphql_query` and `graphql_mutation` do not go through HTTP again. `graph.Executor` is a second
gqlgen executor over the same resolvers, configured by the same function as
`graph.NewHandler` — the query cache, the complexity limit of 60, and GraphQL
introspection only in `dev-mode` — so the two entry points cannot drift apart.
Outside `dev-mode` a `__schema` query through `graphql_query` is refused like
one through `/graphql`; `graphql_introspection` is how an MCP caller reads the
schema, and it shows only the caller's part. Its context carries the
request's `appreq.Request`, so `checkAdmin`, the audit buffer and every command's
transaction work exactly as they do for the SPA. A response with `errors` comes
back as a tool error (`isError: true`); an `ErrorPayload` is data and does not.

**A refused document teaches.** Models write fields that do not exist. When the
response has errors, the tool appends what the model needs to fix the
document: the SDL of every type the errors name (from the caller's own view,
so a non-admin learns nothing of the admin namespace), the nearest names by
spelling for a field, argument or type that does not exist, and the topic to
read. `graph.SchemaView.Explain` does it from the error messages.

**A long answer is cut on the server.** Some hosts cut nothing, so the server
does: past `STARTERKIT_MCP_RESPONSE_CHARS` (default 20000) the answer stops on a
character boundary, and its last line says where it was cut, out of how much,
and how to narrow the document: fewer fields, or fewer nodes.

## The SDK

`github.com/modelcontextprotocol/go-sdk`, the official one, maintained by the
protocol's own organisation. It has a stable v1 API, implements the current
spec's streamable HTTP transport, and ships `auth.RequireBearerToken`, which is
the seam the key check plugs into. `mark3labs/mcp-go` was the community choice
before it and is still alive; picking the official SDK means the transport
follows the spec without us following it.

The handler runs **stateless** (`StreamableHTTPOptions.Stateless`). Each POST
is a complete exchange with no session kept in memory, which is the same
property the panel has towards browsers: nothing to expire, nothing lost on a
restart, any instance can answer. It also means the tool handlers run in the
HTTP request's own context, which is what carries the caller's identity to the
resolvers. `JSONResponse` is on, so a call answers `application/json` instead of
opening a stream; none of the tools stream.

## The path of a call

```
client ──▶ traefik: router `mcp`, no forward-auth (docker/traefik/dynamic/gate.yml)
       ──▶ cmd/server/http.go: base+"/mcp" bypasses withRequest
       ──▶ withMCPRequest          an appreq.Request, no gate identity
       ──▶ auth.RequireBearerToken(verifyMCPKey)
             └─ authenticatemcpkey.Resolve: sha256(key) → mcp_keys + users + admins + user_bans
                  unknown, revoked, owner banned → 401, audited as denied:mcp
                  otherwise → last_used_at = now, at most once a minute; the owner becomes the request's identity
       ──▶ limitBody (1 MiB) ──▶ mcpserver: four tools
       ──▶ flushAudit
```

The MCP path skips `withRequest` because that middleware asks `gateauth` to
verify the `Authorization` header as the gate's ID token, and an MCP key is not
one: it would be refused as a stale token before the key check ran.

## Keys

`mcp_keys` holds `name`, `user_id`, `key_hash`, `created_at`, `last_used_at` and
`revoked_at`. It never holds the key.

- **Create** (`createMCPKey`) mints 32 random bytes, prefixes them `mcp_` so a
  secret scanner can tell what it found, stores their SHA-256 and returns the
  key in the payload. That payload is the only place the key ever exists
  outside the caller. A fast hash is right here: the input is random and 256
  bits long, so there is nothing for a slow hash to protect against, and the
  lookup has to run on every call.
- **The owner is the principal.** A key acts as the admin who created it, and
  the owner's admin grant and ban are read on every call through the same join
  `DBGetIdentityByOIDC` uses. Removing the owner's admin takes their keys out of
  the admin namespace on the next call; banning them refuses their keys
  outright.
- **Revoke** (`revokeMCPKey`) sets `revoked_at`. The row stays, so the page
  still says who held the key and when it was last used, and a refused call
  names it in the audit log.
- **Last used** is accurate to a minute. `DBMarkMCPKeyUsed` writes only when
  `last_used_at` is empty or more than a minute old
  (`authenticatemcpkey.LastUsedGranularity`), so a client calling in a tight
  loop does not take SQLite's single writer on every request. The page can say
  "used in the last minute", never "used at 12:03:41".
- **List** (`admin.mcpKeys(first, after)`) is a connection like every other
  list in the kit ([pagination.md](pagination.md)), newest key first, and
  `admin.mcpKey(id)` reads one.
- **Delete** (`deleteMCPKey`) removes the row. A deleted key's next call is an
  unknown key.
- **Audit.** Every tool call writes `mcp:<tool>` with the key's id, and the
  document for `graphql_query` and `graphql_mutation`. A mutation run through `graphql_mutation` is
  audited a second time by `checkAdmin`, exactly as it would be from the SPA.
  Every refusal writes `denied:mcp` with the reason.

The audit log records a mutation's *variables*, not its response, so the key
that `createMCPKey` returns does not reach `audit_log.detail`. The input is a
name. The day an input field carries a secret, [graphql.md](graphql.md) says
what to add.

The admin page is `assets/ui/mcpkeys`: a catalog, a create page opened from the
catalog's add link that shows the key once with a copy button, and an item page
with Revoke and Delete in its tools.

## One wall, on purpose

Everywhere else the panel sits behind two independent gates: Traefik's
forward-auth, and the panel verifying the ID token itself
([traefik-forward-auth.md](traefik-forward-auth.md)). The MCP route has one.
Its callers are programs with a key, not browsers with a session, so
oauth2-proxy could only refuse them; the router has no forward-auth middleware,
and the key check in the panel is the only thing between the network and the
API.

This is accepted for the kit, and only for the kit: it is a demonstration of
the pattern, not a deployment. What it costs: a bug in `verifyMCPKey` or
`authenticatemcpkey` is a way in, with no second wall to catch it, and a stolen
key is the owner's full authority until somebody revokes it. What bounds it:
the route is one exact path, the key is 256 random bits, the owner's authority
is read live, and every call is in the audit log with the key's id.

A project that exposes this endpoint for real adds a second wall, independent
of the panel's code. Two ways:

- **A separate entrypoint for machines.** Serve `/mcp` on its own host or port,
  not the browser origin, and put the limits there that a program caller can
  live with and a browser could not: an IP allowlist of where callers run, a
  rate limit per source, and mTLS if the callers can hold a client
  certificate. Traefik does all three as middlewares on the `mcp` router. A
  stolen key then also needs a position on an allowed network.
- **Workload identity in place of keys**, the next section. Forging a caller
  then needs the issuer's signing key, which the panel never holds.

## Replacing keys with workload identity

A stored key is a shared secret: whoever holds it is the caller, it does not
expire, and rotating it means a person creating a new one and redeploying the
caller. When the caller runs on a platform that can sign a statement of who it
is — a Kubernetes service account token, a cloud instance identity, SPIFFE, a
CI provider's OIDC token — the panel can verify that statement instead and keep
no secret at all.

The recipe, and the seams it touches:

1. **Configure an issuer and an audience.** Two settings in
   `internal/appconfig`: the issuer whose JWKS signs the caller's token, and the
   audience the token must carry — the MCP endpoint's own URL is the usual
   choice, so a token minted for another service is refused here.

2. **Verify a JWT in place of a key.** `verifyMCPKey` in `cmd/server/mcp.go` is
   the `auth.TokenVerifier` the SDK middleware calls. Replace it with one that
   verifies the token's signature against the issuer's JWKS, plus `iss`, `aud`
   and `exp`. The kit already does this for the gate: `gateauth.New` takes an
   issuer and an audience and builds a go-oidc verifier, and a second
   `gateauth.Client` for the workload issuer is most of the work. Its
   `Identity` insists on a verified email, which a workload token does not
   carry, so split that check out or give the workload client its own claims
   struct. Return `Expiration` in the `TokenInfo` and drop
   `AllowMissingExpiration`: a signed token has an `exp`, and the middleware
   should enforce it.

3. **Map claims to a principal.** An account here is already
   `(oidc_issuer, oidc_subject)` in `users`, so a workload is just another row:
   its issuer and its `sub`. `DBGetIdentityByOIDC` resolves it, admin and ban
   included, with no new table. A workload is granted admin the same way a
   person is, in `admins`, and banned the same way. Create the row on first
   sight, as `signinbyoidc` does for a person, or require an admin to add it
   first if unknown workloads must not get an account at all.

4. **Delete what the key needed.** `authenticatemcpkey`, `internal/mcpkey`, the
   `mcp_keys` table and its three mutations, and `assets/ui/mcpkeys` go. The
   audit detail names the subject instead of a key id. Keep both for a while
   only if existing callers still hold keys, with the verifier trying the shape
   it recognises; then delete the old one.

What it buys:

- **No stored secret.** The database holds who may call, never something that
  lets you call.
- **Rotation for free.** The issuer rotates its signing keys and publishes them
  in its JWKS, and go-oidc follows; callers get fresh short-lived tokens from
  their platform. Nobody creates, copies or revokes anything.
- **A second wall.** Forging a caller now takes the issuer's signing key, which
  lives in a different process under a different credential. Breaking the panel
  does not hand it over, and stealing a token gives an attacker minutes, not
  forever.

What it costs: the caller's platform must issue tokens the panel can reach the
JWKS of, and a token's lifetime is now the gap between withdrawing a workload's
right at the issuer and it taking effect — the same gap [auth-dex.md](auth-dex.md)
describes for people, closed the same way, by reading authority live from
`admins` and `user_bans`.

## A service that is also its own CLI

A program that calls this API from a shell — a script, a scheduled job, a model
driving a terminal — is easier to write against a command than against HTTP and
JSON. The service binary can be that command: `starterkit graphql '{ viewer {
role } }'` posting to `/mcp` or `/graphql` with a key from the environment.
There are two ways to get it onto the caller's machine.

**The same binary, run with a subcommand, copied next to the caller.** At start
the service copies `os.Executable()` into a directory the caller also mounts,
and the caller runs it from there. `main` dispatches on `os.Args[1]` before it
reads the service's configuration.

- The CLI is always exactly the server's version, because it is the server.
  There is no compatibility matrix, and a schema change cannot leave an old
  client behind.
- No network step and no download URL: the file is there before the caller
  starts.
- It needs a shared filesystem and the same OS and architecture on both sides,
  and the CLI carries the whole server's weight.

**Served by the service at a download URL.** `GET <BasePath>/cli` answers the
binary (one per OS and architecture if they differ), and the caller fetches it
when it needs it.

- Works across machines, operating systems and architectures, and nothing has
  to share a disk.
- Fetching is an explicit step that can fail, be cached or be skipped, so a
  caller can run a CLI older than the server; it should check the server's
  version and refetch.
- The download is a route, so it needs the same thought as any other: who may
  fetch it, and whether the binary it serves is the one that was built.

Choose the copy when the caller always runs beside the service on the same
machine, and lockstep matters more than reach. Choose the download when callers
are elsewhere, or there are several platforms. Either way the CLI is a client of
the same API and authenticates like any other caller; it is a convenience, never
a way around the key check.

The kit does not ship either, because it has no caller that would run one, and
a subcommand nobody runs is a mechanism that does nothing
([rules.md](rules.md)). The project that gets such a caller adds the subcommand
and one of the two delivery paths in the same change.
