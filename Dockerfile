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

# The theme and widgets come from airkit, a mam package that follows its main
# branch: the pack line names it, and mam clones it on the first build that
# needs an $airkit_ module. MAM_PULL_DISABLED only stops mam from pulling the
# pinned workspace above; a missing package is still cloned.
RUN echo 'pack airkit git \https://github.com/alexesDev/airkit.git' >> .meta.tree

# The workspace mounts this project's UI under its package name. Locally the
# same thing is a symlink; see docs/frontend-mol.md.
COPY assets/ui /mam/starterkit

# The bundle's own half of docs/build-info.md: the commit the SPA compares with
# every X-Build-Commit it receives. Hex or nothing, because it lands in source.
ARG GIT_COMMIT=
RUN echo "${GIT_COMMIT}" | grep -Eqx '[0-9a-f]*' \
 && printf 'namespace $ {\n\texport const $starterkit_buildinfo_commit: string = %s\n}\n' "'${GIT_COMMIT}'" \
    > /mam/starterkit/buildinfo/commit/commit.ts

RUN MAM_PULL_DISABLED=1 node node_modules/.bin/mam starterkit/app

FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui /mam/starterkit/app/- ./assets/ui/app/-

# The commit and the build time the admin footer shows, stamped with -X because
# this stage has no .git for Go to read a revision from. Left unset, the build
# is a dev build and says so. See docs/build-info.md.
ARG GIT_COMMIT=
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /out/starterkit ./cmd/server

FROM gcr.io/distroless/static:nonroot

COPY --from=build /out/starterkit /starterkit

# 7301 is the app, 7302 is Prometheus. Neither is meant to face the internet
# directly — see docs/traefik-forward-auth.md.
EXPOSE 7301 7302

USER nonroot:nonroot
ENTRYPOINT ["/starterkit"]
