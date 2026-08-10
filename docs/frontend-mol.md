# Frontend

The SPA is [$mol](https://mol.hyoo.ru). It talks to one GraphQL endpoint through
generated, typed wrappers — no fetch calls written by hand, no client cache to
configure.

## Layout

```
assets/ui/
  app/          $starterkit_app — the root, a $mol_book2_catalog
  catalog/      $starterkit_catalog — the shared catalog base
  labeler/      $starterkit_labeler_{id,name,moment} — a titled cell
  paging/       $starterkit_paging — the keyset cursor every list pages by
  gql/          the request runtime, and generated schema types
  users/
    catalog/    the list
    show/       one user, with the ban and admin controls
  auditlog/     the audit log with keyset paging
```

A $mol component is a directory holding a file triple:

| File | What it is |
|---|---|
| `x.view.tree` | the declarative structure — what is rendered, what is bound |
| `x.view.ts` | the behaviour, as a subclass overriding properties from the tree |
| `x.view.css.ts` | styles, optional |

Names are global and derived from the path: `assets/ui/users/catalog/` is
`$starterkit_users_catalog`. There are no imports — the builder resolves every
`$`-prefixed name to its directory.

## Rows are labelers, not strings

A catalog row is a `$mol_row` of labeled cells, so each column carries its own
title:

```tree
menu_link_content* / <= User_item* $mol_view
    sub /
        <= Row* $mol_row
            minimal_height 24
            sub /
                <= Id* $starterkit_labeler_id
                    value <= row_id_string* \
                <= Email* $starterkit_labeler_name
                    title \Email
                    value <= row_email* \
                <= Seen* $starterkit_labeler_moment
                    title \Last sign-in
                    value <= row_last_sign_in* \
```

`$starterkit_labeler_moment` renders `-` for an empty value, so a missing date
does not leak into the UI as blank space. It does **not** format: the server
already did. See [rules.md](rules.md).

## Time comes formatted

There is no date library in this SPA and there should not be one. Every
timestamp in the schema is a `Timestamp` object, and a screen asks for the shape
it renders:

```graphql
createdAt { formatted(layout: "2006-01-02 15:04") }
lastSignInAt { formatted }          # RFC3339, the schema default
createdAt { unix }                  # a number, for sorting or diffing
```

`assets/ui/users/catalog/list.graphql` uses an explicit layout;
`assets/ui/users/show/data.graphql` takes the bare default, which is ISO, which
$mol consumes with no parsing. The TypeScript reads `row.createdAt.formatted`
and nothing else happens on the client. `layout` is Go's reference layout —
`"02.01.2006"` renders `10.08.2026`.

## GraphQL: one file, one operation

Write a `.graphql` next to the component that uses it. `make codegen` turns each
into a typed function whose name is its path:

```
assets/ui/users/catalog/list.graphql ->  $starterkit_users_catalog_list(vars?)
```

```ts
@$mol_mem
data() {
    return $starterkit_gql_make_map($starterkit_users_catalog_list().admin.users.nodes)
}
```

The call is synchronous. $mol runs it under a fiber, so the component reads the
result as a plain value and re-renders when it changes; there is no promise, no
loading flag, no `useEffect`.

The operation name sent to the server is overwritten with the path-derived one,
so the name in the audit log matches the file that produced it.

### The one convention to know

**Every mutation refetches every query on the page.** A query subscribes to a
shared marker; a mutation bumps it; every memoised query re-runs. `$mol_mem`
compares results, so unchanged data does not re-render.

This is why a mutation screen does not have to tell its catalog to refresh. Opt
a call out with `{ revalidate: false }` when that is genuinely wrong.

### Errors are data

A union comes back discriminated by `__typename`, so a refused ban is rendered,
not thrown:

```graphql
... on BanUserPayload { userId }
... on ErrorPayload { message byFields { name value } }
```

`assets/ui/gql/fielderrors.ts` turns `byFields` into a per-control message, so a
form puts each failure under the input that caused it instead of showing one
banner.

## Shared behaviour lives in a class, not in every screen

Paging is the same on every list, so it is written once in
`assets/ui/paging/paging.ts` as `$starterkit_paging<Node>` — a plain
`$mol_object` rather than a view, so a screen that is not a page can use it too.

A screen supplies `fetch` and nothing else:

```ts
@$mol_mem
paging() {
    const view = this

    return new (class extends $starterkit_paging<Entry> {
        override page_size() { return view.page_size() }

        override fetch(before: number | null) {
            return $starterkit_auditlog_list({ limit: this.page_size(), before }).admin.auditLog
        }
    })()
}
```

The base owns the cursor list, memoises each page by the cursor it started from,
concatenates them, and answers `has_next()` / `more()`. `@$mol_mem` on the
instance means a page already fetched is not fetched again, while the shared
invalidation marker still refetches everything after a mutation.

An anonymous subclass rather than `$mol_object.make({...})`: `make` is generic
over the class, so it cannot carry the `<Node>` parameter, and the node type is
what makes the rows type-safe. The type itself is derived from the query so it
can never drift:

```ts
type Entry = ReturnType<typeof $starterkit_auditlog_list>['admin']['auditLog']['nodes'][number]
```

One shape detail worth knowing: codegen types a nullable GraphQL field as
*optional* (`endCursor?: number | null`), so `$starterkit_paging_page` declares
it optional too. Declaring it merely nullable makes the generated result
unassignable, and the error is a wall of structural-mismatch text.

## Codegen

`assets/codegen/ui/` is a graphql-codegen preset. Three files: `graphqlgen.js`
(config), `preset.js` (one output per input), `molplugin.js` (emits the typed
seam).

The one thing worth knowing about `molplugin.js`: it escapes every `$` in the
stock plugins' output as `$`. The $mol builder scans sources for
`$`-prefixed names to build its module graph, and GraphQL variables (`$limit`)
and fragment-masking keys would otherwise read as phantom module references. TS
treats the escape as the same character, so types and runtime strings are
unchanged. `$starterkit_gql_ref` in `assets/ui/gql/gql.ts` is the other half of
that contract — it is what the generated code for a fragment file refers to, and
it is why the escape appears in hand-written code too.

The schema is read from the checked-in `internal/graph/schema.graphqls` — no
running server needed. Change the schema, run `make gqlgen && make codegen`, and
the frontend stops compiling where it no longer matches.

## Building

$mol builds inside a [mam](https://github.com/hyoo-ru/mam) workspace, which
vendors `$mol` itself and resolves modules by directory rather than by import.
So the workspace has to contain this project's UI under its package name.

Set that up once:

```bash
git clone --depth 1 https://github.com/hyoo-ru/mam.git ../mam
cd ../mam && npm install && npm install jsdom
```

Then link this project in — `make ui-link` does exactly this:

```bash
ln -s "$PWD/assets/ui" ../mam/starterkit
```

`MAM_WORKSPACE` overrides the location. From then on:

```bash
make ui                        # one-off build -> assets/ui/app/-/web.js
make mol-dev                   # dev server on :9080, rebuilds on save
```

In development the root belongs to that dev server. The panel claims exactly one
prefix — every route it registers lives under `appconfig.BasePath`, default
`/_system/starterkit` — and Traefik hands everything else to mam, so a saved
`.view.tree` or `.view.ts` is visible on refresh with no build step. Both routes
carry the same forward-auth gate, and that gate is load-bearing: the panel's own
identity check cannot cover pages the panel no longer serves.

`make mol-dev` runs `scripts/mol-dev.sh`, which reuses a dev server already
listening on :9080 rather than dying on `EADDRINUSE` — mam hardcodes that port,
and one instance covers every project linked into the workspace, because it
serves the *whole* workspace. That is also what the dev root hands out: whatever
the mam server answers is reachable in dev, behind the gate. Accepted for
development; the e2e stand and production never mount the route. Without the dev
server running the UI is a 502, while the panel keeps answering under its
prefix.

Two consequences worth holding onto:

- `make ui` leaves the inner loop. The built bundle is still what the Docker
  image embeds and what the e2e stand serves — it is just no longer how you see
  an edit.
- Dev and e2e serve the SPA by different paths: dev is the live workspace
  through mam, e2e and production are the bundle embedded in the binary.
  Anything that differs between the two surfaces only in e2e or production,
  never in the dev loop — `make ui` before shipping frontend changes is the
  cheap way to catch that class early.

`jsdom` is there because the builder also runs a node test bundle that needs it.

The build runs `tsc` over the whole dependency closure, so a query that does not
match the schema fails the build rather than the browser. `assets/ui/**/-/` is
gitignored, so a fresh clone has no bundle until `make ui` runs.

## Serving

`assets/embed.go` embeds the built bundle. `/ui/*` is served as static files;
everything else outside the panel's base path returns `index.html`, so
client-side deep links survive a reload. Routing is `$mol_state_arg` — the URL
hash — with no router to configure.

Both paths are behind the identity check, so the shell is never served to an
anonymous visitor.

## How the SPA finds the API

The GraphQL endpoint comes from a global the shell sets:

```js
window.starterkit_settings = { graphqlUrl: '/_system/starterkit/graphql' }
```

The panel's `index.html` loads that object as `<BasePath>/settings.js` — a real
file served by the panel, because the CSP has no `unsafe-inline` for scripts and
an inline snippet would be silently blocked. `gql.ts` falls back to the default
above when nothing sets the global, which is exactly the dev case: `test.html`
is served by mam, which the panel cannot inject into. The fallback mirrors
`appconfig.BasePath`'s default — change one and the other must follow, and the
failure mode is loud: every query answers 404.
