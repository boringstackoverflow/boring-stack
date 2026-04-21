#!/usr/bin/env bash
# Boring Stack reference deploy script.
# Build a static Go binary, ship it, swap atomically, verify, roll back if broken.
# Set HOST + HEALTHZ_URL via env vars, or edit the defaults below.

set -euo pipefail

HOST=${HOST:-deploy@your-vps.example.com}
REMOTE=${REMOTE:-/home/deploy/app}
HEALTHZ_URL=${HEALTHZ_URL:-https://your-domain.example.com/healthz}
SHA=$(git rev-parse --short HEAD 2>/dev/null || echo dev)

echo "→ build $SHA"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w -X main.version=$SHA" -o app .

echo "→ upload"
scp -q app "$HOST:$REMOTE/app.new"

echo "→ swap + restart (keeps app.prev for rollback)"
ssh "$HOST" "cd $REMOTE && cp -f app app.prev 2>/dev/null || true; \
             mv app.new app && sudo systemctl restart app"

echo "→ health check (5 retries, 2s apart)"
for i in 1 2 3 4 5; do
    sleep 2
    if curl -fsS --max-time 3 "$HEALTHZ_URL" >/dev/null 2>&1; then
        echo "✓ live: $SHA"
        exit 0
    fi
    echo "  retry $i..."
done

echo "✗ health check failed, rolling back"
ssh "$HOST" "cd $REMOTE && mv app.prev app && sudo systemctl restart app"
exit 1
