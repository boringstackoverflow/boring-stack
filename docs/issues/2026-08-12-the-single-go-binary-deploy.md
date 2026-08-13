---
title: "The single Go binary deploy"
date: 2026-08-12
summary: "Build one Linux binary, copy it over SSH, swap it atomically, restart systemd, and roll back when the health check fails. The entire release path is 26 executable lines."
draft: false
---

A deploy does not have to be a platform.

For one Go app on one VPS, it can be one artifact crossing one SSH connection. Build the binary, copy it beside the running binary, rename it into place, restart the service, and ask the public URL whether it came back.

That is the whole path. The production-minded version in this repo is **37 lines total, 26 executable**. The extra lines buy version stamping, five health-check attempts, and automatic rollback.

## The deploy

Comments removed, this is [`templates/deploy.sh`](https://github.com/boringstackoverflow/boring-stack/blob/main/templates/deploy.sh):

```bash
set -euo pipefail

HOST=${HOST:-deploy@your-vps.example.com}
REMOTE=${REMOTE:-/home/deploy/app}
HEALTHZ_URL=${HEALTHZ_URL:-https://your-domain.example.com/healthz}
SHA=$(git rev-parse --short HEAD 2>/dev/null || echo dev)

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w -X main.version=$SHA" -o app .

scp -q app "$HOST:$REMOTE/app.new"

ssh "$HOST" "cd $REMOTE && \
  { if [ -f app ]; then cp -f app app.prev; fi; } && \
  mv app.new app && sudo systemctl restart app"

for i in 1 2 3 4 5; do
  sleep 2
  if curl -fsS --max-time 3 "$HEALTHZ_URL" >/dev/null 2>&1; then
    echo "live: $SHA"
    exit 0
  fi
done

ssh "$HOST" "cd $REMOTE && mv app.prev app && sudo systemctl restart app"
exit 1
```

There is no image, registry, release service, remote build, or deployment agent. Your laptop needs Go, SSH, and curl. The server needs systemd and permission for the deploy user to restart one service.

## Build once

`GOOS=linux GOARCH=amd64` builds for the VPS instead of whatever machine you are sitting at. `CGO_ENABLED=0` keeps the result self-contained when your dependencies permit it. Copy that file to a fresh Ubuntu box and it runs without installing Go.

`-trimpath` removes local source paths. `-s -w` strips debug tables to make the upload smaller. `-X main.version=$SHA` puts the Git commit into the binary, assuming the app exposes a `main.version` variable. A `/version` endpoint or startup log can then answer the most useful deploy question: *what code is actually running?*

If you need CGO—SQLite drivers based on `mattn/go-sqlite3` are the common reason—cross-compilation needs a Linux C toolchain. The boring alternatives are to build on Linux in CI or use a pure-Go SQLite driver. Do not discover this for the first time during a production deploy.

## Never overwrite the running binary

The upload goes to `app.new`, not `app`.

Uploading directly onto the live path can fail with `text file busy` while the process is running. If the old process is stopped, an interrupted transfer can instead leave a partial executable at the path systemd expects. Neither is an interesting failure mode.

`mv app.new app` is atomic when both files are on the same filesystem. Observers see the old file or the new file, never half of either. The currently running process also keeps using the old inode until systemd stops it, so replacing the path does not mutate the code already in memory.

Before the swap, `cp app app.prev` keeps one rollback target. This is not release history. It is enough to answer, “the new binary is broken; put back the one that was running two minutes ago.” Git remains the history.

The SQLite database is not part of this swap. That is deliberate—and it is why schema migrations must be backward-compatible with the previous binary if you expect automatic rollback to work. A binary rollback cannot undo a destructive migration.

## Restart, check, roll back

`systemctl restart app` gives the old process its SIGTERM window, then starts the new file at the same path. Caddy stays up. During the gap, requests can receive a 502. For a small app, that brief interruption is often an honest trade for a deploy path one person can understand completely.

The script waits two seconds and calls the public HTTPS health endpoint. Public matters: checking `localhost:8080` proves the process is listening, but not that Caddy routing, DNS, or TLS still works. We have already shipped that exact mistake.

Five attempts tolerate a slow startup. If all five fail, the script restores `app.prev`, restarts again, and exits non-zero. A failed deploy should be loud and should leave the last known binary running.

A useful `/healthz` checks that the process can serve requests and open its database. It should not call every external dependency. If Resend is down, your application may still be healthy enough to accept signups and queue mail for later.

## What this does not solve

This is not zero-downtime deployment. It does not coordinate ten servers, drain a load balancer, approve releases, retain twenty artifacts, or undo database migrations. It assumes one operator, one host, one service, and a few seconds of acceptable disruption.

When those assumptions stop being true, add the missing property—not a platform-shaped bundle of properties you do not need. Run two ports behind Caddy for zero downtime. Put versioned binaries in object storage for longer rollback history. Add CI when more than one person ships. Move to an orchestrator when you have a fleet worth orchestrating.

Keep the artifact. A Go binary that works under systemd also works in a container later. Starting without the container does not close that door.

## This week's receipt

The complete script is in [`templates/deploy.sh`](https://github.com/boringstackoverflow/boring-stack/blob/main/templates/deploy.sh). Copy it, change three variables, and read every line before giving the deploy user permission to restart the service.

This is issue #8. Issue #7 said the restore drill would be next. We changed the order to clear the deploy-script promise from issue #6 first. The restore drill is still owed; changing the calendar does not make the debt disappear.

What is the smallest deploy script you actually trust—and which failure made its least obvious line necessary?
