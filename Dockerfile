# The $mol bundle is built in its own stage, inside a mam workspace, because
# mam resolves modules by directory rather than by import.
FROM node:24-alpine AS ui

RUN apk add --no-cache git

# Pinned, and not to a branch: this stage runs the workspace's own npm install
# scripts, so an unreviewed upstream commit executes at build time and can emit
# a bundle that exfiltrates whatever the admin types into it.
ARG MAM_COMMIT=d02777f535e59ae48c52e314b112b2b3fff7c35f

WORKDIR /mam
RUN git init -q . \
 && git remote add origin https://github.com/hyoo-ru/mam.git \
 && git fetch -q --depth 1 origin "${MAM_COMMIT}" \
 && git checkout -q FETCH_HEAD \
 && npm install --no-audit --no-fund \
 && npm install --no-audit --no-fund jsdom

# The workspace mounts this project's UI under its package name. Locally the
# same thing is a symlink; see docs/frontend-mol.md.
COPY assets/ui /mam/starterkit

RUN node node_modules/.bin/mam starterkit/app

FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui /mam/starterkit/app/- ./assets/ui/app/-

# The revision the panel shows in its footer, and the only thing that puts one
# in the binary. Left unset it says `unknown` rather than naming a revision it
# is not.
ARG GIT_COMMIT=unknown
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.GitCommit=${GIT_COMMIT}" -o /out/starterkit ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/starterkit /starterkit

# 7301 is the app, 7302 is Prometheus. Neither is meant to face the internet
# directly — see docs/traefik-forward-auth.md.
EXPOSE 7301 7302

USER nonroot:nonroot
ENTRYPOINT ["/starterkit"]
