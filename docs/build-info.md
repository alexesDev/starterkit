# Build info

**The admin always shows which build it is running: the commit, and when it was
built. A build that was not told its commit is labelled `dev`, never given a
made-up hash.** And when the server answering is a different build from the
bundle in the tab, the tab says so and offers a reload.

## Where the commit comes from: `-ldflags -X`, not `debug.ReadBuildInfo`

Go can stamp a revision on its own: `go build` inside a git checkout records
`vcs.revision` and `vcs.time`, and `runtime/debug.ReadBuildInfo` reads them back.
That looks like the simpler path and it is not one here. The image is built by
`docker build`, whose context is the `.dockerignore` allowlist, and `.git` is
not on it — so Go has nothing to read a revision from and silently records
none. Allowing `.git` to fix that ships the whole history into a build stage.

So the revision is handed to the build, the way any deploy hands one over:

```dockerfile
ARG GIT_COMMIT=
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /out/starterkit ./cmd/server
```

```bash
docker build --build-arg GIT_COMMIT="$(git rev-parse HEAD)" -t app .
```

`main.GitCommit` and `main.BuildTime` are empty package-level strings, because
`-X` can only set one of those. `BuildTime` is RFC 3339 in UTC; a value that is
not refuses to start the binary rather than show a wrong date.
`internal/buildinfo.New` turns the pair into one `Info`, and nothing else in
the server reads the globals.

Pass the full 40-character commit. The footer shows the first seven; the full
one is what a click copies and what the repository link names.

## A build without a commit is `dev`

`make build`, `make dev` and `go run` set no `-X`, and a `docker build` without
`--build-arg GIT_COMMIT` sets an empty one. All of them are dev builds: the API
answers `commit: null`, the footer shows a `dev` chip, and no
`X-Build-Commit` header is sent. There is no `unknown` and no placeholder hash
— a footer that names a revision is a claim, and only a build that was told one
makes it.

## What the admin sees

```graphql
type AdminQuery {
  build: Build!
}

type Build {
  commit: String        # null in a dev build
  builtAt: Timestamp    # null when the build was not stamped
  commitUrl: String     # null unless STARTERKIT_REPO_URL is set and the build has a commit
}
```

`Build` binds onto `buildinfo.Info` and has no resolvers of its own. It sits
under `admin`, so only admins read it, as the footer did before.

The footer, `$starterkit_buildinfo_footer`:

- the short commit is a button; a click copies the full commit — through the
  Clipboard API on a secure origin, and by selecting a hidden field and copying
  it on a plain-http one, where that API does not exist;
- with `STARTERKIT_REPO_URL` set (`https://github.com/org/app`, no trailing
  slash), an icon beside it opens `<repo>/commit/<sha>`; the server builds the
  link, so the client never learns a repository's URL layout;
- the build time, already formatted by the server;
- a dev build shows the `dev` chip in place of all of it.

## Version skew: a quiet reload bar

A tab opened before a deploy keeps running the old bundle against the new
server. The bundle knows its own commit, the server says its own on every
answer, and the tab compares the two on answers it was receiving anyway — no
polling, no endpoint of its own.

- **The bundle's commit** is compiled in. The image's `ui` stage writes
  `assets/ui/buildinfo/commit/commit.ts` from the same `GIT_COMMIT` before mam
  builds, refusing anything that is not hex because the value lands in source.
  The checked-in file holds `''`, which is what dev and `make ui` build.
- **The server's commit** is the `X-Build-Commit` header, set by
  `buildinfo.Info.Announce` on every GraphQL response that passed the identity
  check. A dev build sets none.
- **The comparison** is in `$starterkit_gql_post`: each response's header goes
  to `$starterkit_buildinfo.served`, and `stale()` is true only when both sides
  name a commit and they differ.

When it is true, a bar at the bottom of the screen reads "A new version is
deployed" with one action, Reload. Nothing is blocked and nothing reloads by
itself — the person may be halfway through a form.

## Tests

- `internal/buildinfo` — the header on a stamped build, none on a dev build;
  the commit link; a malformed build time refusing to start.
- `assets/ui/buildinfo/buildinfo.test.ts` — the skew rule, run by mam's node
  test bundle.
- `e2e/auth.spec.ts` — the footer and `admin.build` name the commit the runner
  built with, the header carries it, and a response rewritten to another commit
  raises the bar, which a reload clears.
