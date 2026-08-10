#!/usr/bin/env bash
# End-to-end tests against the e2e stand: the same compose file as `make up`,
# in its own compose project and behind its own hostnames, plus the panel
# built from the Dockerfile — the suite tests the artefact that ships.
#
#   scripts/e2e.sh                      # full run
#   UPDATE_SNAPSHOTS=1 scripts/e2e.sh   # rewrite e2e/__snapshots__, then read the diff
#   scripts/e2e.sh --ui                 # arguments pass through to playwright
#
# The stand shares the machine with `make dev` and collides with none of it:
# e2e has its own Traefik on 127.0.0.82, where every *.e2e.sk.localbox name
# resolves, so both stands can run at once and neither run's teardown touches
# the other's ingress.
#
# Containers and volumes outlive a run that was killed halfway. Everything the
# suite owns is therefore purged at the START of a run — that is what makes a
# run reproducible — and again at the end, which is just being a good
# neighbour.
set -euo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Exported: the specs stop, start and SIGTERM this container by name, and a
# name they defaulted to themselves would be a second copy of it.
export PANEL_CONTAINER=starterkit-e2e-panel
PANEL_VOLUME=starterkit-e2e-panel-data
PANEL_PORT=7401
METRICS_PORT=7402
BASE_PATH=/_system/starterkit

purge_e2e_state() {
	scripts/stack.sh purge e2e >/dev/null 2>&1 || true
	rm -rf e2e/.cache
}

cleanup() {
	local status=$?

	if [ "$status" -ne 0 ]; then
		echo "[e2e] panel log:"
		docker logs --tail 100 "$PANEL_CONTAINER" 2>&1 || true
		echo "[e2e] stack status:"
		docker compose -p starterkit-e2e ps 2>&1 | head -30 || true
	fi

	purge_e2e_state

	exit "$status"
}
trap cleanup EXIT INT TERM

purge_e2e_state

scripts/stack.sh up e2e

if [ ! -d node_modules/@playwright/test ]; then
	npm install
fi
npx playwright install chromium --with-deps >/dev/null 2>&1 || npx playwright install chromium

# Handed to the build the way a deploy hands it over, so e2e/auth.spec.ts can
# read it back out of the panel's footer — the only check that the revision
# reaches the binary at all.
PANEL_COMMIT=$(git rev-parse --short HEAD)
export PANEL_COMMIT

docker build --build-arg GIT_COMMIT="$PANEL_COMMIT" -t starterkit-e2e-panel:local .

# Re-asserted right before the panel starts, minutes after stack.sh's own
# waits: the panel's boot gate probes the issuer and dies on a non-answer, so a
# hop that drifted while the image built should fail here with its name
# instead of there with a riddle.
curl -sf -o /dev/null "http://dex.e2e.sk.localbox/dex/.well-known/openid-configuration" || {
	echo "[e2e] dex stopped answering through traefik" >&2
	exit 1
}

# In the image's home directory because the container runs as nonroot and that
# is the one path a fresh named volume inherits nonroot ownership on. No
# restart policy: e2e/drain.spec.ts SIGTERMs this container and then asserts it
# stayed down and exited 0.
docker run -d --name "$PANEL_CONTAINER" --network host \
	-v "$PANEL_VOLUME":/home/nonroot \
	-e STARTERKIT_ADDR=127.0.0.1:$PANEL_PORT \
	-e STARTERKIT_METRICS_ADDR=127.0.0.1:$METRICS_PORT \
	-e STARTERKIT_PUBLIC_URL=http://panel.e2e.sk.localbox \
	-e DATABASE_URL=sqlite:/home/nonroot/e2e.sqlite3 \
	-e STARTERKIT_OIDC_ISSUER=http://dex.e2e.sk.localbox/dex \
	-e STARTERKIT_OIDC_CLIENT_ID=starterkit \
	-e STARTERKIT_BOOTSTRAP_ADMIN=admin@example.com \
	-e STARTERKIT_LOG_LEVEL=info \
	-e STARTERKIT_DEV_MODE=true \
	-e STARTERKIT_TRUSTED_PROXIES=127.0.0.1/32,::1/128 \
	-e STARTERKIT_METRICS_INTERVAL=1s \
	-e STARTERKIT_SHUTDOWN_DELAY=5s \
	-e STARTERKIT_SHUTDOWN_TIMEOUT=15s \
	starterkit-e2e-panel:local >/dev/null

for _ in $(seq 60); do
	if curl -sf -o /dev/null "http://127.0.0.1:$PANEL_PORT$BASE_PATH/healthz"; then
		break
	fi
	sleep 1
done
curl -sf -o /dev/null "http://127.0.0.1:$PANEL_PORT$BASE_PATH/healthz" || {
	echo "[e2e] panel never became healthy" >&2
	exit 1
}

export APP_URL="http://panel.e2e.sk.localbox"
export PANEL_URL="http://127.0.0.1:$PANEL_PORT"
export METRICS_URL="http://127.0.0.1:$METRICS_PORT"

# Here rather than as a spec, because a deploy can run the same file against
# its own origin and the list of paths has to be one list. Run with the panel
# up on purpose: a refusal from an origin with nothing behind it is not
# evidence of a wall.
scripts/refuse_system_paths.sh "$APP_URL"

npx playwright test "$@"

echo "[e2e] all specs passed"
