# Traefik forward-auth

> Can Traefik sit in front of the panel and guarantee that a person only gets in
> through the IdP — even if there is a bug in the panel itself?

Yes. That is what this stack is.

## How it works

```
browser
  │
  ▼
traefik 127.0.0.81:80 (dev) — one per stand, e2e's is 127.0.0.82:80
  │  forwardAuth: every request to the panel host first goes to that stand's
  │  oauth2-proxy /oauth2/auth
  │
  ├─ not 2xx ──▶ 401, rewritten to the Dex sign-in page. Nothing upstream is dialled.
  │
  └─ 2xx ─────▶ panel.sk.localbox/_system/starterkit* → panel 127.0.0.1:7301
                panel.sk.localbox/*  → mam dev server 127.0.0.1:9080 (mol-dev.yml)
                panel.e2e.sk.localbox → panel 127.0.0.1:7401, through e2e's own Traefik
```

Traefik's `forwardAuth` middleware asks oauth2-proxy about **every** request and
only opens a connection to the panel after a 2xx. oauth2-proxy holds its own
session cookie, signed with its own secret, and validates against Dex.

The property that matters: this gate shares nothing with the panel. Not a
process, not a secret, not a line of code. A missing `requireIdentity` on a new
route, a bug in our own header handling — none of it gets a request past
Traefik, because Traefik never asks the panel anything.

`e2e/gate.spec.ts` proves the unauthenticated half on every run, against a stand
where the panel container is *stopped*. A gate that holds while the thing behind
it is absent cannot be passing for any reason other than the gate.

## Running it

```bash
make up          # traefik, dex, oauth2-proxy on docker compose; waits until healthy
make mol-dev     # own terminal — the mam dev server the root is routed to
make dev
```

There is no profile to opt into. The dev stand's Traefik listens on
`127.0.0.81:80`, the panel on `127.0.0.1:7301`, and the panel's loopback bind is
load-bearing: the entrypoint is the only way in. Open <http://panel.sk.localbox>.

The panel cannot usefully be reached without the gate. It has no login flow of
its own, so a request straight to `127.0.0.1:7301` arrives with no ID token and
is answered 401.

## An address per stand

Dev and e2e do not share anything. Each has its own Traefik, and the two are
told apart by the address they bind: `127.0.0.81` for dev, `127.0.0.82` for e2e,
both on port 80. All of `127.0.0.0/8` is loopback, so the same port on two
addresses is two independent sockets — which is the property that makes
`scripts/stack.sh down e2e` an operation on e2e alone. The names follow the
addresses: `*.sk.localbox` resolves to the first, `*.e2e.sk.localbox` to the
second, one dnsmasq line per zone (see the README).

The `Host()` rules are still there, so a request arriving on a stand's address
under some other name is answered 404 rather than served by whichever router
happened to be least specific.

## Who discovers what

Nothing is discovered: every upstream sits on a static loopback port per stand —
panel 7301/7401, oauth2-proxy 4380/4381, Dex 5756/5757 — and
`docker/traefik/dynamic/gate.yml` routes them by name. A `forwardAuth`
middleware address is file-provider config and could not follow a dynamic port
anyway, and the dev panel is a host process `make dev` owns, not a container.

## Why host networking

Every stack container runs with `network_mode: host` and binds loopback. The
panel listens on `127.0.0.1`, deliberately, and a bridged container cannot reach
the host's loopback. Host networking plus names that resolve to the stand's own
loopback address is what keeps the Dex issuer URL —
`http://dex.<zone>/dex` — byte-identical for the browser, for oauth2-proxy and
for the panel; a mismatched issuer is a login loop. A host-networked container
resolves through the host's own stub resolver, so the one dnsmasq rule covers it
too. Linux only; on Docker Desktop the panel would need to bind `0.0.0.0` and be
firewalled instead.

## Configuration worth knowing

Traefik binds one `web` entrypoint on the stand's address, port 80, and sets
**no** `forwardedHeaders.trustedIPs`: Traefik overwrites `X-Forwarded-*` on
every request, so a client cannot dictate the IP the panel records. Listing
`127.0.0.1` there would defeat it, because in development the browser is itself
the loopback peer.

`docker/traefik/dynamic/gate.yml`:

- the `/oauth2/` router has priority 100 so it out-ranks the panel catch-all —
  the sign-in endpoints cannot sit behind the middleware that redirects to them
- the `oauth-errors` middleware turns oauth2-proxy's bare 401 into the sign-in
  page, carrying the original URL so the user lands back where they started. It
  rewrites 401 only: the panel answers 403 with the reason a session was refused
  (`access revoked: <reason>`), and sending that to the sign-in page would loop a
  banned user with nothing explaining why
- `authResponseHeaders` passes the authenticated identity to the panel.
  `Authorization` carries the ID token and is the panel's *only* identity
  source. It is verified, not trusted: the signature is checked against the
  issuer's JWKS on every request (`internal/gateauth/gateauth.go`). The unsigned
  `X-Auth-Request-*` headers are ignored on purpose

`docker/traefik/dynamic/mol-dev.yml` is the dev stand's routing: the panel keeps
its base path (`/_system/starterkit*`, priority 60) and the mam dev server takes
the rest (priority 50, shadowing gate.yml's panel catch-all), both behind the
same middlewares, so the admin UI stays gated even though the panel no longer
serves it. What scopes this file to dev is the `STARTERKIT_MAM_ROUTING` guard in
its template: only the dev stand's Traefik gets the flag, and e2e keeps
gate.yml's production shape — the built bundle through the panel catch-all. See
[frontend-mol.md](frontend-mol.md).

`docker/dex/config.yaml` is rendered by the Dex image's own entrypoint from the
container's environment, so a config change means re-running `make up`, not
editing a running container.

## Known gaps

**`/_system/starterkit/healthz` is behind the gate on the panel hosts.** An
external prober needs either the direct panel port or a higher-priority router
without the middleware.

**`OAUTH2_PROXY_EMAIL_DOMAINS=*`** lets any Dex-authenticated identity through
the gate. Authority is then limited by the panel's own `admins` table. To make
the gate itself enforce an allowlist, use `--authenticated-emails-file` or set a
real domain.

**The oauth2-proxy cookie secret and the Dex client secret are committed** as
defaults in `docker/compose.yml`. Fine for a loopback stand, wrong the moment
this is deployed — override them from the environment or move them to a secret
store.

**Dex still has `enablePasswordDB` and static passwords.** Remove both before
any real deployment; see [auth-dex.md](auth-dex.md).

`scripts/refuse_system_paths.sh` is the check that keeps the first two honest
over time: it asks a public origin for every path that must never answer a
stranger and fails if any of them does. `scripts/e2e.sh` runs it on every run,
and a deploy should run the same file against its own origin.
