# No -X main.GitCommit here: a local build is not a deployed one and its footer
# says `dev`. Only the image build is told a revision — see the Dockerfile.
LDFLAGS := -s -w

.PHONY: dev mol-dev run build build-amd64 test lint fmt generate gqlgen sqlc moq \
        db-new db-up db-down db-status ui ui-link codegen \
        up down status purge

# ---- run ----------------------------------------------------------------

# The panel alone, against an already-built bundle. Refuses to start against
# half a stand: the preflight names what is missing instead of the panel
# failing its boot gate later.
dev:
	scripts/stack.sh preflight dev
	go tool github.com/air-verse/air

# The mam dev server, which Traefik serves the SPA from in development —
# reuses one already listening on :9080. Run next to `make dev`.
mol-dev:
	MAM_WORKSPACE="$(MAM_WORKSPACE)" ./scripts/mol-dev.sh

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -o ./tmp/starterkit -ldflags="$(LDFLAGS)" ./cmd/server

build-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./tmp/starterkit-amd64 -ldflags="$(LDFLAGS)" ./cmd/server

# ---- checks -------------------------------------------------------------

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	./internal/db/list_queries.sh
	go vet ./...
	go tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint run

# ---- codegen ------------------------------------------------------------

# Full loop, in dependency order: schema -> Go -> TypeScript.
generate: sqlc gqlgen moq codegen

gqlgen:
	go tool github.com/99designs/gqlgen generate

sqlc:
	go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate
	./internal/db/fix_write_queries.sh

moq:
	go generate ./...

codegen:
	npm run codegen:ui

# ---- database -----------------------------------------------------------

DBMATE = go run github.com/amacneil/dbmate/v2

db-new:
	$(DBMATE) new $(name)

db-up:
	$(DBMATE) up

db-down:
	$(DBMATE) down

db-status:
	$(DBMATE) status

# ---- frontend -----------------------------------------------------------

# $mol builds inside a mam workspace that has this project's UI linked in under
# its package name. One-time setup, then `npm start` in that workspace serves a
# live-rebuilding bundle. See docs/frontend-mol.md.
MAM_WORKSPACE ?= ../mam

# rm first: `ln -sfn` onto an existing real directory silently creates the link
# INSIDE it, and mam then builds an empty package into the workspace.
ui-link:
	rm -rf "$(MAM_WORKSPACE)/starterkit"
	ln -s "$(CURDIR)/assets/ui" "$(MAM_WORKSPACE)/starterkit"
	@echo "linked $(MAM_WORKSPACE)/starterkit -> assets/ui"

ui: ui-link
	cd "$(MAM_WORKSPACE)" && node node_modules/.bin/mam starterkit/app

# ---- stack --------------------------------------------------------------

up:
	scripts/stack.sh up dev

down:
	scripts/stack.sh down dev

status:
	-docker compose -p starterkit-dev ps
	-docker compose -p starterkit-e2e ps

# Everything the stand accumulated, so the next `up` behaves like a first
# install: the bootstrap admin is promoted again on first login and the audit
# log starts empty. The dex volume goes with it, which is what re-mints the
# signing keys.
#
# Your browser still holds oauth2-proxy's cookie afterwards, and the gate will
# still consider you signed in — clear site data for panel.sk.localbox if you
# want the true first-visit path.
purge:
	scripts/stack.sh purge dev
