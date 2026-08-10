#!/usr/bin/env bash
# The one way a stand comes up or goes down.
#
#   scripts/stack.sh up dev        # Traefik + Dex + oauth2-proxy, wait until healthy
#   scripts/stack.sh preflight dev # fast probes only — `make dev` refuses to start on failure
#   scripts/stack.sh origin dev    # the addressing this stand comes up on
#   scripts/stack.sh down dev
#   scripts/stack.sh purge e2e     # down + volumes + persisted state
#
# Both stands run the same compose file (docker/compose.yml); the stand
# argument picks the compose project, the loopback address the stand owns and
# the ports, so `down` is an operation on one stand and cannot reach the
# other's ingress. Both hold port 80, because all of 127.0.0.0/8 is loopback
# and two processes may share a port on different addresses.
set -euo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

ACTION=${1:?usage: stack.sh <up|down|purge|preflight|origin> <dev|e2e>}
STAND=${2:?usage: stack.sh <up|down|purge|preflight|origin> <dev|e2e>}

# Deliberately not settable from the environment. The address and the ports are
# what tell the two stands apart, and a stand brought up against someone else's
# numbers is a stand that quietly took over their ingress. Pick numbers no other
# stack on the machine already holds; the zone is named separately from the
# stand so the address a stand answers on can move without renaming the stand.
case "$STAND" in
dev)
	ADDR=127.0.0.81
	ZONE=sk.localbox
	PANEL_PORT=7301
	OAUTH_PORT=4380
	DEX_PORT=5756
	MAM_ROUTING=true
	;;
e2e)
	ADDR=127.0.0.82
	ZONE=e2e.sk.localbox
	PANEL_PORT=7401
	OAUTH_PORT=4381
	DEX_PORT=5757
	MAM_ROUTING=false
	;;
*)
	echo "unknown stand: $STAND" >&2
	exit 1
	;;
esac

PROJECT="starterkit-$STAND"
OPS_PORT=8190
PANEL_ORIGIN="http://panel.$ZONE"
ISSUER_URL="http://dex.$ZONE/dex"

compose() {
	STARTERKIT_BIND_ADDR="$ADDR" \
	STARTERKIT_ZONE="$ZONE" \
	STARTERKIT_OPS_PORT="$OPS_PORT" \
	STARTERKIT_PANEL_PORT="$PANEL_PORT" \
	STARTERKIT_OAUTH_PORT="$OAUTH_PORT" \
	STARTERKIT_DEX_PORT="$DEX_PORT" \
	STARTERKIT_MAM_ROUTING="$MAM_ROUTING" \
	STARTERKIT_PANEL_ORIGIN="$PANEL_ORIGIN" \
	STARTERKIT_ISSUER_URL="$ISSUER_URL" \
	docker compose -p "$PROJECT" -f docker/compose.yml "$@"
}

wait_http() {
	local url=$1 label=$2 tries=${3:-60}

	for _ in $(seq "$tries"); do
		if curl -sf -o /dev/null "$url"; then
			return 0
		fi
		sleep 1
	done

	echo "[stack] $label never answered at $url" >&2
	return 1
}

probe() {
	local url=$1 label=$2

	if ! curl -sf -o /dev/null --max-time 5 "$url"; then
		echo "[stack] $label is not answering at $url — run: make up" >&2
		return 1
	fi
}

# Every hostname of this stand has to resolve to the address its Traefik holds,
# or the failure lands minutes later inside a container as a discovery timeout.
# One line of dnsmasq covers the whole zone — see README.md, "Names".
require_names() {
	local got
	# `|| true`: getent exits 2 when the name does not resolve, which under
	# `set -e` would kill this script before it could print the diagnostics
	# below — the unresolved-name case, the one worth explaining, would be the
	# one that exited silently.
	got=$(getent hosts "panel.$ZONE" | awk '{print $1; exit}' || true)

	if [ "$got" = "$ADDR" ]; then
		return 0
	fi

	if [ -z "$got" ]; then
		echo "[stack] panel.$ZONE does not resolve — see README.md, \"Names\"" >&2
	else
		echo "[stack] panel.$ZONE resolves to $got, not $ADDR: that is another stack's address or a stale pin" >&2
	fi

	exit 1
}

# The stand's addressing is written down a second time in .env — and .gitignore
# hides that file, so a change of origin reaches the repository and .env.example
# and never reaches anyone's own copy. Only the dev stand has a .env: e2e runs
# the panel as a container with its own environment.
#
# Compared against what this script derived rather than against a pattern: the
# panel verifies the gate's ID token against its own STARTERKIT_OIDC_ISSUER, so
# an issuer that differs from the gate's by one character is a panel that
# refuses every session, and the message says only that a token failed to
# verify.
require_env_addressing() {
	if [ "$STAND" != dev ] || [ ! -f .env ]; then
		return 0
	fi

	local wrong="" name value want

	for name in STARTERKIT_PUBLIC_URL STARTERKIT_OIDC_ISSUER; do
		value=$(sed -n "s|^$name=||p" .env | tail -1)
		[ -n "$value" ] || continue

		case "$name" in
		STARTERKIT_PUBLIC_URL) want="$PANEL_ORIGIN" ;;
		STARTERKIT_OIDC_ISSUER) want="$ISSUER_URL" ;;
		esac

		[ "$value" != "$want" ] || continue

		wrong="$wrong[stack]   $name=$value"$'\n'"[stack]     want: $want"$'\n'
	done

	if [ -n "$wrong" ]; then
		echo "[stack] .env does not name the origin this stand serves:" >&2
		printf '%s' "$wrong" >&2
		echo "[stack] .env is gitignored, so a change in the repository never reached it — compare it with .env.example." >&2
		exit 1
	fi
}

wait_healthy() {
	wait_http "http://$ADDR:$OPS_PORT/ping" "traefik" 60
	wait_http "$ISSUER_URL/.well-known/openid-configuration" "dex (at the issuer)" 90
	wait_http "http://127.0.0.1:$OAUTH_PORT/ready" "oauth2-proxy" 120

	echo "[stack] $STAND stand is up: $PANEL_ORIGIN"
}

case "$ACTION" in
up)
	require_names
	require_env_addressing
	compose up -d --quiet-pull
	wait_healthy
	;;

preflight)
	require_env_addressing
	probe "http://$ADDR:$OPS_PORT/ping" "traefik"
	probe "$ISSUER_URL/.well-known/openid-configuration" "dex"
	probe "http://127.0.0.1:$OAUTH_PORT/ready" "oauth2-proxy"
	;;

origin)
	echo "panel_origin=$PANEL_ORIGIN"
	echo "issuer_url=$ISSUER_URL"
	;;

down)
	compose down --remove-orphans
	;;

# Everything the stand accumulated, so the next `up` behaves like a first
# install: the bootstrap admin is promoted again on first login and the audit
# log starts empty. The dex volume goes with it, which is what re-mints the
# signing keys.
#
# Your browser still holds two cookies afterwards. The panel's own now names a
# user id that no longer exists and is dropped on the next request, but
# oauth2-proxy's outlives this and the gate will still consider you signed in —
# clear site data for panel.sk.localbox if you want the true first-visit path.
purge)
	compose down --remove-orphans --volumes
	if [ "$STAND" = "dev" ]; then
		rm -f data.sqlite3 data.sqlite3-wal data.sqlite3-shm
		echo "[stack] purged: dev database, dex volume"
	else
		docker rm -f starterkit-e2e-panel >/dev/null 2>&1 || true
		docker volume rm -f starterkit-e2e-panel-data >/dev/null 2>&1 || true
		echo "[stack] purged: e2e stack, panel container and volume"
	fi
	;;

*)
	echo "unknown action: $ACTION" >&2
	exit 1
	;;
esac
