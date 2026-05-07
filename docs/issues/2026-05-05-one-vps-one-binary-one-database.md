---
title: "One VPS, one binary, one database"
date: 2026-05-05
summary: "The Boring Stack in one sitting — what each piece is, why it's there, and what it costs. The mental model you need before you read anything else here."
draft: false
---

If you remember nothing else from this site, remember this:

**One VPS. One Go binary. One SQLite file. Continuous backup to object storage. Caddy for HTTPS. systemd to keep it alive. A shell script to deploy.**

That is the whole stack. The rest of this issue is what each piece is doing, what it replaces, and what it costs at scale.

If that sentence is enough — go read [the manifesto](/manifesto.html), install [the skill](/#install), and get on with shipping. If you want to know why each piece is there, keep reading.

## The picture

```
browser
  │
  ▼
┌──────────────────────────────────────────┐
│  one $5 VPS (Hetzner cx22, 4GB RAM)      │
│                                          │
│  Caddy ──► Go binary ──► SQLite (WAL)    │
│                             │            │
│                             ▼            │
│                         Litestream       │
└─────────────────────────────┬────────────┘
                              │
                              ▼
                        Cloudflare R2
                     (continuous backup)
```

That's the architecture diagram. It fits in a tweet. It also runs the entire control plane for [Borela](https://borela.dev/) in production: marketing site, magic-link auth, dashboard, locked v1 protocol, dev mailer.

## What each piece is doing

### The VPS

A $5/mo Hetzner cx22 (4 GB RAM, 2 vCPU, 40 GB disk, 20 TB transfer). Pick a region close to your users. SSH in. It's a Linux box. You can read every file on it.

This is not a compromise. A single VPS is a forcing function. It tells you: if you can't fit the app here, you owe yourself an honest answer about why, before you reach for K8s.

### The Go binary

One file. `CGO_ENABLED=0 go build -o app .` produces a statically-linked Linux binary. `scp` it to the VPS. `systemctl restart`. Done. No interpreter, no runtime, no `node_modules`, no container layer.

The HTTP router is `http.ServeMux` from the standard library. Since Go 1.22 it supports method-prefixed routes (`mux.HandleFunc("POST /v1/whatever", handler)`). You no longer need a router library.

### The database

SQLite, in WAL mode, in a single file at `/home/<app>/data/app.db`.

WAL mode means: readers do not block writers, writers do not block readers. You get real concurrent reads with one writer. For 99% of side projects, that ceiling is irrelevant — modern SQLite handles 10k+ writes/sec on an NVMe-backed VPS.

The big objection — "but what about backups?" — is the next piece.

### Litestream

[Litestream](https://litestream.io/) watches your SQLite WAL and streams it to object storage in near-real-time. If your VPS dies, you can restore the database to the last second on a fresh box with one command:

```bash
litestream restore -o /var/lib/app/app.db s3://bucket/app.db
```

We use Cloudflare R2 because it has zero egress fees. The bill is $0.12/mo at our scale. You can also use S3, B2, GCS — anything S3-compatible.

You do not have a backup until you have restored from it. Run a restore drill the day you set this up. Then run another one in a month. Future issues will go deep on this.

### Caddy

Caddy is a web server that handles HTTPS automatically. You put your domain in the Caddyfile, Caddy talks to Let's Encrypt, you have TLS forever. No certbot, no cron, no nginx config.

The full Caddyfile for a one-app box:

```
api.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

That's it. That's the whole web server config.

### systemd

systemd runs your binary. It restarts it if it crashes (`Restart=on-failure`). It captures stdout/stderr into the journal, where you read it with `journalctl -u app -f`. It's already on every modern Linux box. It's the runtime you didn't have to install.

A hardened service file is about 22 lines:

```ini
[Service]
Type=simple
User=app
ExecStart=/usr/local/bin/app
Restart=on-failure
RestartSec=2

NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/home/app/data
PrivateTmp=yes
```

The full template is [in the repo](https://github.com/boringstackoverflow/boring-stack/blob/main/templates/app.service).

### The deploy script

10 lines. Build, ship, install, restart, smoke-test:

```bash
#!/usr/bin/env bash
set -euo pipefail
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app .
scp app root@$VPS:/tmp/app.new
ssh root@$VPS '
  sudo install -m 755 /tmp/app.new /usr/local/bin/app
  sudo systemctl restart app
  sleep 1
  curl -fsS http://127.0.0.1:8080/healthz
'
```

That's the whole deploy pipeline. It's readable when tired. It works on every box that has SSH. It does not require a Docker registry, a CI vendor, or a YAML file.

## What this replaces

| You'd reach for | Boring Stack picks | Why |
|---|---|---|
| Postgres | SQLite + WAL + Litestream | One file. Concurrent reads. Continuous backup. Migration to Postgres later is `pgloader` + a connection string. |
| Vercel / Render / Fly | Hetzner + Caddy | $5/mo at every scale. Auto-HTTPS. No platform lock-in. |
| Prisma / GORM / SQLAlchemy | stdlib + sqlc | Type-safe Go generated from `.sql` files. No runtime layer. You can read your own queries. |
| Docker / docker-compose / K8s | systemd + one binary | Already on every Linux box. `Restart=on-failure` is one line. |
| GitHub Actions deploy pipeline | 10-line shell script | Readable when tired. No vendor. No YAML. |
| Datadog / New Relic | `journalctl` + healthz + uptime check | Free for the first year of any side project. Add observability when there's something to observe. |

## What this costs

The exact bill for the [Borela](https://borela.dev/) control plane this month:

```
hetzner cx22          $4.59
cloudflare r2         $0.12
boringstack.org dns   $1.25
resend (free tier)    $0.00
────────────────────────────
total                 $5.96/mo
```

That's the whole control plane: marketing site, magic-link auth, dashboard, the locked v1 protocol that babysits real production apps, and the dev mailer. One box. One database. Six dollars.

Future issues will publish this number every month. When it changes, I'll explain why.

## When this is wrong

Boring Stack is not the right answer for every project. The skill ships a "When NOT to use this" section, and so does this site. The honest list:

- More than ~100GB of database. SQLite is fine; the operational story for 100GB+ files starts to suck.
- Multi-region active-active. SQLite does not replicate to multiple writers. If you need that, you need Postgres + something like Aurora or CockroachDB.
- Compliance where downtime matters in human-life terms. Medical, life-safety. Use a platform with an SLA you can sue.
- More than ~50 engineers on the codebase. The monolith starts to creak. Boundaries become political.
- Real-time collaboration on shared state. Operational transforms or CRDTs are their own architecture; SQLite is the wrong primitive.

If your project doesn't match any of those, Boring Stack probably fits. The point isn't to never use Postgres or never use Docker. The point is to **make complexity a conscious choice** instead of the default.

## The deal

This is issue #2 of a weekly newsletter. The deal is one practical field note every Tuesday — what shipped, what broke, how the bill changed, which defaults held up, and what the latest restore drill found.

Next Tuesday: **"SQLite is a database. Stop apologizing."** The full case for SQLite in production, with benchmarks, with the failure modes, with the migration path when you outgrow it.

[Subscribe](/#newsletter) if you want it. [Star the repo](https://github.com/boringstackoverflow/boring-stack) if you want to follow along quietly. [Install the skill](/#install) if you're ready to try it on a project.

Reply with your stack. I read every reply.
