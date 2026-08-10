#!/usr/bin/env bash
# The mam dev server serves the WHOLE workspace, so one instance already covers
# every project linked into it — including this one. Starting a second is what
# produces `EADDRINUSE :::9080`, and mam has no way to move its websocket
# listener off that port. So: reuse whatever is already there.
set -euo pipefail

PORT=${MAM_PORT:-9080}
WORKSPACE=${MAM_WORKSPACE:-../mam}

if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then
	echo "mam dev server already listening on :${PORT} — reusing it"
	echo "bundle: http://127.0.0.1:${PORT}/starterkit/app/-/web.js"
	exit 0
fi

cd "$WORKSPACE"
exec npm start
