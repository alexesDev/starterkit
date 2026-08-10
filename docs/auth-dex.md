# Authentication and authorization

Three separate questions, answered in three separate places:

| Question | Answered by |
|---|---|
| Who is this person? | the customer's IdP, reached by oauth2-proxy |
| Do they have an account here? | `users`, keyed on `(oidc_issuer, oidc_subject)` |
| What may they do? | `admins`, `user_bans` |

## The login flow

The panel does not have one. That is the design.

```
browser → Traefik → forwardAuth → oauth2-proxy /oauth2/auth
                                    └─ no session? 401 + sign-in page
                                       └─ authorization code flow against the IdP

  once oauth2-proxy holds a session, its /oauth2/auth answers 200 with
  Authorization: Bearer <id_token>, which Traefik copies onto the upstream
  request (docker/traefik/dynamic/gate.yml, authResponseHeaders)

browser → Traefik → panel
  └─ gateauth.Identity: verify the token against the issuer's JWKS, plus iss,
     aud and exp; require email_verified
  └─ DBGetIdentityByOIDC(issuer, subject): one indexed read for email, admin, ban
     └─ first time seen? signinbyoidc.Resolve creates the user and audits it
```

One authorization-code exchange, run by a component whose job that is. The panel
holds no session, so there is nothing to steal, expire or revoke, and the
customer configures one OIDC client rather than two.

The token is verified rather than trusted. A header on a reachable port can be
written by anyone who can reach that port; a signature over the issuer's JWKS
cannot. That is what makes the panel a wall of its own rather than a restatement
of the gate's decision.

**Reaching the panel without the gate is not a way in.** No token means no
identity, and the panel answers 401 saying so.

## Signing out

The session belongs to oauth2-proxy, so only its endpoint can end one. The
`signOut` mutation records the event in the audit log and returns `redirectUrl`
— `STARTERKIT_SIGN_OUT_URL`, default `/oauth2/sign_out` — which the client then
navigates to. The path is configuration because it belongs to whatever runs in
front of the panel.

The same URL is how the panel escapes a stale session. oauth2-proxy never
re-verifies the ID token its session carries, so when the IdP's signing keys are
gone — rotated past the overlap window, or regenerated because Dex lost its
storage — the gate keeps forwarding a token nothing verifies any more. The panel
rejects it (`rejected an identity from the gate`, at WARN) and answers a browser
navigation with a redirect to the sign-out URL, so the dead session ends itself
and the next request runs a fresh login. Anything that is not a navigation —
`POST /graphql` above all — still gets its 401, because redirecting an API call
to a sign-in page hands the client HTML where it expects JSON.

`signinbyoidc` also bootstraps the first admin: if `admins` is empty and the
email matches `STARTERKIT_BOOTSTRAP_ADMIN`, that user is promoted. Without it a
fresh install has no way in, since only an admin can grant admin. Once any admin
exists the rule never fires again.

## Noticing a sign-in without holding a session

`users.last_signin_at` is the token's `iat`, the last time the panel saw a
*new* one. The panel has no session, so every request looks alike and "someone
signed in" is not an event it can otherwise observe; oauth2-proxy presents the
same ID token for the life of its own session, so `iat` advancing means a fresh
authentication.

The comparison lives inside the `update` statement as well as in Go, so two
concurrent first requests carrying the same new token cannot both decide they
were first: exactly one matches a row and writes the audit entry.

Not `auth_time`, which would be the correct claim — Dex does not emit it. And
enabling token refresh at the proxy would make this count renewals too.

## Pointing this at a corporate identity provider

This is what Dex is for. It federates: it speaks OIDC to us and something else
to whatever the company already runs. Deploying into a corporate environment
means editing `docker/dex/config.yaml` connectors and changing no Go code.

| Connector | Covers |
|---|---|
| `ldap` | OpenLDAP, FreeIPA, Active Directory |
| `saml` | any SAML 2.0 IdP |
| `microsoft` | Entra ID / Azure AD |
| `google` | Google Workspace, restrictable by domain and group |
| `github`, `gitlab` | restrictable by org and team |
| `oidc` | Okta, Auth0, Keycloak, anything generic |

The panel never learns which of these was used. It receives a verified email.

Three things to change before any real deployment:

**Delete `enablePasswordDB` and `staticPasswords`.** They are a development
convenience, and leaving them in place is a permanent way in that bypasses the
corporate IdP entirely.

**Give Dex storage that survives it.** Dex keeps its signing keys in its storage
backend and mints new ones when that storage starts empty — and every live
oauth2-proxy session then carries an ID token no current key verifies, so every
user's next visit lands on the sign-out redirect described above. The dev stand
accepts this trade on purpose: its keys live in a named docker volume that
`make purge` deletes, which doubles as the quickest way to exercise the
stale-session path. An installation must not — losing Dex's storage signs out
everyone at once, so it is as durable a requirement as the panel's own database.

**Decide how admin is granted.** Authorization comes from the local `admins`
table: the IdP says who you are, this database says what you may do. It is read
on every admin request rather than at login, so revoking admin takes effect on
the next one. The separation is deliberate — a compromised IdP still cannot
grant admin here. The cost is that grants are managed here rather than in the
corporate directory.

**An account is `(issuer, subject)`, not an address.** Email and name are
profile data, refreshed from the token on every sign-in. That matters for
exactly one scenario, and it is not hypothetical in a company: someone leaves,
their mailbox is handed to the next employee, and that person signs in with a
different subject. Keyed on email they would land on the old row and inherit
whatever it was granted. `email` has no unique constraint for the same reason —
two subjects sharing an address is a fact to record, not an error to raise.

If a company wants directory groups to drive it instead, Dex can pass a `groups`
claim; read it in `gateauth` and map it in `signinbyoidc`. That is not
implemented here, and note the trade it makes: it hands the IdP the power to
grant admin.

## One lookup resolves the caller

Every request turns the token's `(issuer, subject)` into the full identity in a
single indexed query:

```sql
select u.id, u.email, u.name, u.created_at, u.last_signin_at,
       case when a.user_id is null then 0 else 1 end as is_admin,
       coalesce(b.reason, '') as ban_reason
  from users u
  left join admins a    on a.user_id = u.id
  left join user_bans b on b.user_id = u.id
 where u.oidc_issuer = ? and u.oidc_subject = ?;
```

Identity, authority and revocation in one statement. Consequences:

- **Revoking admin takes effect on the next request.** There is no cached role.
- **A ban takes effect on the next request**, and its reason is right there, so
  the panel answers `403 access revoked: <reason>` instead of a bare login page.
- **No cache to invalidate.** The cost is one indexed lookup per request.

### What is deliberately not solved

An individual session cannot be revoked here, because the panel does not hold
one — that is oauth2-proxy's to end, and until it does, a token it forwards
stays valid until it expires. What the panel can revoke is *authority*, and
authority is read live: a ban or a removed grant applies on the next request no
matter how fresh the token is. Set a short token lifetime at the IdP if the gap
matters.

## The admin boundary

`Query.admin` and `Mutation.admin` resolve a marker object *after* checking the
caller is an admin, so every field beneath is gated by construction rather than
by remembering to write a check. See [graphql.md](graphql.md).

Every crossing is recorded in `audit_log`:

- every **denial**, always
- every successful **mutation**
- successful queries are *not* recorded — an entry per admin page load would
  bury the entries worth reading

## Why audit writes bypass the transaction

`app.WriteAudit` uses `appState.baseWrite`, never the request's transaction
handle. This is deliberate and easy to get wrong.

A denied admin mutation returns an error, so `runInTx` rolls the transaction
back. Had the audit row been written inside that transaction it would be rolled
back too — and the record of the attempt would vanish exactly when it matters
most.

Inside a request `WriteAudit` only buffers, and the middleware flushes once the
transaction is released; writing immediately would deadlock against the single
write connection the open mutation is holding. `flushAudit` also detaches from
the request context, so a client that disconnects cannot take the record of what
it did with it.

For the same reason `WriteAudit` returns nothing: a failure is logged, not
propagated. An unwritable audit row must not turn a clean 403 into a 500.

## Two gates, not one

`requireIdentity` in `cmd/server/http.go` is the application's own gate, and it
is the *second* one: Traefik refuses unauthenticated requests before they reach
the process at all, so a route that forgot `requireIdentity` is still not
exposed. See [traefik-forward-auth.md](traefik-forward-auth.md).

## Secrets at rest

There are none. The panel verifies the gate's ID token and holds no session, no
client secret and no signing key, and nothing in the schema is a credential. A
project built from this kit that adds one is making a design decision, not a
convenience — write down where it lives, what opens it, and what a full
compromise of the process reaches.
