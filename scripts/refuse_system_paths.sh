#!/usr/bin/env bash
# Every path that must never answer a stranger, asked of the public origin.
#
#   scripts/refuse_system_paths.sh http://panel.e2e.sk.localbox
#   scripts/refuse_system_paths.sh https://panel.example.com
#   scripts/refuse_system_paths.sh https://dex.example.com /dex/.well-known/openid-configuration
#
# scripts/e2e.sh runs it against the e2e stand on every CI run, and the deploy
# of a project built from this kit can run the same file against its own
# origin. The list below is the one copy; adding a path here covers both.
#
# What it asserts is that the gate answers and refuses — not merely that the
# request failed. A probe that only looked for "not 200" would pass loudest on
# an origin that was down, which is the one state in which it proves nothing, so
# the first thing checked is that the origin is up.
#
# For a gated origin, up means `/` answers 401. Not every published origin is
# gated — Dex's own hostname cannot be, it is the sign-in page — so a second
# argument names a path that must answer 2xx instead. That keeps the check on
# the origin being alive, which is the property this rests on, and drops only
# the assumption that a wall is what answers.
#
# The paths are internal surfaces, and each of them is somebody's whole session
# if it answers: /metrics is the panel's Prometheus listener, /debug/pprof is a
# heap and a goroutine dump. None of them is routed today. This exists so that
# none of them starts being routed by accident when the next thing is published.
set -euo pipefail

ORIGIN=${1:?usage: refuse_system_paths.sh <origin> [alive-path]}
ORIGIN=${ORIGIN%/}
ALIVE_PATH=${2:-}

# Two families. The first is a workload's own operational surface, which is the
# one that travels: a service published for the routes people use drags these
# along unless something says otherwise. The second is the control plane, which
# binds loopback here and is on this list so that it stays that way — a router
# added for one of them would be answered by this script, not by an incident.
PATHS=(
	/metrics
	/debug/pprof
	/debug/pprof/heap
	/_system/starterkit/healthz

	/v1/sys/health
	/v1/status/leader
	/v1/agent/health
	/api/rawdata
	/dashboard/
)

# A body that looks like the thing itself, whatever the status said. A 200 is
# the obvious tell and the one the predecessor alerted on; these catch the case
# where something answers 401 in the headers and the payload anyway.
LEAKS=(
	'# HELP '
	'go_goroutines'
	'goroutine profile'
)

fail() {
	echo "[refuse] $*" >&2
	exit 1
}

# curl prints 000 itself when it never got a status, and exits non-zero saying
# so; the `|| true` is what keeps that from being an unexplained set -e death.
probe() {
	local code=""

	code=$(curl -s -o "$2" -w '%{http_code}' --max-time 15 "$ORIGIN$1") || true

	echo "${code:-000}"
}

body=$(mktemp)
trap 'rm -f "$body"' EXIT

# The origin has to be up, or the whole run below would pass against a host that
# had simply stopped. 401 on / is what an unauthenticated request gets from
# oauth2-proxy through Traefik; an ungated origin names its own live path.
if [ -n "$ALIVE_PATH" ]; then
	alive_status=$(probe "$ALIVE_PATH" "$body")
	case "$alive_status" in
	2*) ;;
	*)
		fail "$ORIGIN$ALIVE_PATH answered $alive_status, not 2xx: this origin is not serving, so nothing below would mean anything"
		;;
	esac
else
	root_status=$(probe / "$body")
	if [ "$root_status" != "401" ]; then
		fail "$ORIGIN/ answered $root_status, not 401: this is not a gated origin, so nothing below would mean anything — name a path that is served instead"
	fi
fi

for path in "${PATHS[@]}"; do
	status=$(probe "$path" "$body")

	case "$status" in
	2*)
		fail "$ORIGIN$path answered $status — it is served to anyone who asks"
		;;
	000)
		fail "$ORIGIN$path could not be reached at all: the origin answered / but not this, which is a probe that proved nothing"
		;;
	esac

	for leak in "${LEAKS[@]}"; do
		if grep -qF -- "$leak" "$body"; then
			fail "$ORIGIN$path answered $status but its body carries '$leak': the status refused and the payload did not"
		fi
	done

	echo "[refuse] $path -> $status"
done

echo "[refuse] $ORIGIN refuses every system path"
