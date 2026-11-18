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

FROM gcr.io/distroless/static:nonroot

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

**The build stage has no `--platform`, and there is no `TARGETOS` or
`TARGETARCH`.** One Dockerfile builds the same way under every builder: the
legacy builder rejects `FROM --platform=$BUILDPLATFORM` when the argument is
empty, and a default makes BuildKit ignore the real build platform. Without it,
`go build` targets the stage's own platform, so the target arguments would do
nothing. The cost: a foreign-architecture image under `buildx` builds under
QEMU, which is slower and needs binfmt registered on the build machine.

**`-h` exits 0.** A probe that only wants to know the image starts — the binary
is there, it is the right architecture, it runs as `nonroot` — runs
`docker run --rm image -h` with no configuration at all. The panel does this
already: `internal/appconfig` parses with `flag.ContinueOnError`, and `main`
treats `flag.ErrHelp` as a clean exit rather than a fatal error. A service
built from this template keeps the same two lines, so the probe is the same for
every image.

## Decision: the base is `gcr.io/distroless/static:nonroot`, not `scratch`

`distroless/static` is `scratch` plus four things a Go service needs, and no
shell or package manager. Each is a line one forgets in a `scratch` image and
only notices at runtime:

- **CA certificates** at `/etc/ssl/certs/ca-certificates.crt`. Without them
  every outbound TLS call fails with `x509: certificate signed by unknown
  authority`.
- **A `nonroot` passwd entry** (uid 65532) in `/etc/passwd` and `/etc/group`,
  with a home directory. Without it `USER nonroot` does not resolve, and the
  easy fix — dropping the `USER` line — runs the service as root.
- **tzdata** under `/usr/share/zoneinfo`, so `time.LoadLocation` works. On
  `scratch` it fails unless the binary embeds `time/tzdata`.
- **A writable `/tmp`** with the sticky bit, so `os.TempDir()` and anything
  buffering to disk have somewhere to write.

`scratch` is fine only for a binary that needs none of those: no outbound TLS,
no time zones, no temporary files, and a numeric `USER` written by hand.
