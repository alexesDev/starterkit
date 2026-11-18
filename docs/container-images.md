# Container images

The panel ships as one static Go binary in a distroless image. `Dockerfile`
builds the $mol bundle in its own stage, embeds it, and copies the binary into
`gcr.io/distroless/static-debian12:nonroot`; `Dockerfile.runtime` wraps a binary
built beforehand by `make build-amd64`.

## A Go service without the SPA

A second binary in a project built from the kit — a worker, a webhook receiver —
has no bundle to embed, so it needs neither the node stage nor the mam
workspace. The whole image is two stages:

```dockerfile
FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service "./cmd/${SERVICE}"

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/service /service

USER nonroot:nonroot
ENTRYPOINT ["/service"]
```

```bash
docker build --build-arg SERVICE=worker -t worker .
DOCKER_BUILDKIT=0 docker build --build-arg SERVICE=worker -t worker .
docker buildx build --platform linux/amd64,linux/arm64 --build-arg SERVICE=worker -t registry.example/worker --push .
```

**`CGO_ENABLED=0`** is what makes the binary static, and a static binary is what
lets the final image have no libc. It is also why the kit's SQLite driver is
`modernc.org/sqlite` — see [data-layer.md](data-layer.md).

**The build stage has no `--platform`.** The common cross-compiling form,
`FROM --platform=$BUILDPLATFORM golang` with `GOOS=$TARGETOS
GOARCH=$TARGETARCH`, depends on BuildKit filling those arguments in. The legacy
builder does not, and it refuses the empty value outright (`"" is an invalid OS
component`); giving the argument a default does not rescue it, because BuildKit
then uses the default instead of the real build platform. Without `--platform`
both builders work with no arguments: the legacy builder builds for the machine
it runs on, and `buildx --platform a,b` runs the build stage once per target, so
`go build` already targets the stage's own platform and needs no `GOOS` or
`GOARCH`. The price is that a foreign platform builds under emulation, which
needs QEMU registered with binfmt on the build machine and is several times
slower. A pipeline that only ever uses BuildKit and wants native-speed cross
builds can take the `$BUILDPLATFORM` form instead, knowing it has given up the
legacy builder.

**`-h` exits 0.** A probe that only wants to know the image starts — the binary
is there, it is the right architecture, it runs as `nonroot` — runs
`docker run --rm image -h` with no configuration at all. The panel does this
already: `internal/appconfig` parses with `flag.ContinueOnError`, and `main`
treats `flag.ErrHelp` as a clean exit rather than a fatal error. A service
built from this template keeps the same two lines, so the probe is the same for
every image.

## Why distroless `static` rather than `scratch`

`scratch` is an empty filesystem, and a Go service quietly needs four things
from one. `distroless/static` is those four things and nothing else — no shell,
no package manager:

- **CA certificates** at `/etc/ssl/certs/ca-certificates.crt`. Without them
  every outbound HTTPS call fails with `x509: certificate signed by unknown
  authority`.
- **A `nonroot` user** (uid 65532) in `/etc/passwd` and `/etc/group`, with a
  home directory. `USER nonroot:nonroot` resolves by name, and anything that
  looks the current user up finds one.
- **tzdata** under `/usr/share/zoneinfo`, so `time.LoadLocation("Europe/Berlin")`
  works. On `scratch` it fails unless the binary embeds `time/tzdata`.
- **A writable `/tmp`** with the sticky bit, so `os.TempDir()` and anything
  buffering an upload to disk have somewhere to write.

Each can be copied into `scratch` by hand from the build stage. That is four
`COPY` lines that drift from upstream, for an image a few megabytes smaller.
