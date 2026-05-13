---
title: "SQLite is a database. Stop apologizing."
date: 2026-05-12
summary: "The full case for SQLite in production: what it actually does well, what WAL mode changes, what Litestream adds, what fails, when to switch to Postgres, and how to migrate when you do."
draft: false
---

There is a particular sentence that shows up in every SQLite-in-production thread:

> "Yeah, but it's not a *real* database."

It is. It has been for two decades. It runs on every iPhone in the world. It runs Expensify. It runs the weather data inside your operating system. It is, by deployment count, the most widely deployed database engine on Earth.

What people mean when they say "real database" is usually one of:

1. *Concurrent writers across machines* — a thing you almost certainly do not need on day one.
2. *A managed-service dashboard* — a thing you don't need at all.
3. *A familiar shape from a job ten years ago* — the actual reason.

This issue is the long version of why SQLite is the right default for the small web app you are about to build, what it does well, what breaks, and what you do on the day you outgrow it.

## What SQLite is

A C library. Not a server. Not a daemon. Not a thing you connect to over a socket. Your Go binary opens a file with `database/sql`, and queries run inside your process. There is no second process to monitor.

This single design choice is what makes everything downstream simple:

- No connection pool to tune.
- No connection limits to hit.
- No socket round-trip per query.
- No "the database is down" failure mode separate from "the app is down."
- No version-skew between your client driver and your server.

A read in SQLite is a function call. The latency floor is microseconds, not milliseconds.

## What WAL mode changes

The default journal mode in SQLite is *rollback*: writers and readers serialize through a single lock. Fine for a CLI tool. Painful the first time you have an HTTP handler reading rows while a background worker is writing.

Set this at startup:

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
```

Three lines. What they buy you:

- **WAL mode**: writes append to a side log; readers see a consistent snapshot of the main database without blocking. Concurrent reads with a single writer — the exact shape of a small web app.
- **synchronous = NORMAL**: writes are durable on commit but skip the extra fsync that *FULL* mode does on every page write. Standard for application workloads; not for "must survive simultaneous power loss to two disks" scenarios.
- **busy_timeout = 5000**: if a write does have to wait for the writer lock, wait up to 5 seconds before failing. Eliminates the "SQLITE_BUSY" errors that scare people off in benchmarks they ran wrong.

Set these once, at open. Forget about them.

After WAL: a modern SQLite on an NVMe-backed VPS handles **10,000+ writes per second** before contention shows up. Reads are bounded by memory bandwidth — for any database that fits in RAM (and most do), they are effectively free.

## What Litestream adds

The honest objection to SQLite-as-production-database is not performance. It's backups.

If your VPS dies and your backup strategy is "I cron'd a `cp` every night," you will lose up to 24 hours of writes. That's a real problem. It's also a problem with a clean answer: [Litestream](https://litestream.io/).

Litestream watches your SQLite WAL and streams every change to object storage in near-real-time. The recovery story:

```bash
litestream restore -o /home/app/data/app.db s3://bucket/app.db
```

One command. Restores to the last second of writes. A typical small DB (hundreds of MB) restores in well under a minute on a fresh box.

We use Cloudflare R2 because it has zero egress fees. The bill at the boringstack.org scale is **$0.00/mo** — under the R2 free tier, even with WAL frames streaming continuously.

The rule that separates "I have a backup" from "I have a backup":

> If you have not restored from your backup in the last 30 days, you do not have a backup.

Add a restore-drill cron that runs weekly into a `/tmp` dir, runs `sqlite3 .schema` and a row count, then deletes it. If the cron fails, you find out before you need it.

## What actually fails

Here are the failure modes that show up in production, ranked by how often they bite people:

### 1. The "I put the DB on NFS" failure

SQLite locking on networked filesystems is broken. Always. Don't put the DB file on NFS, on SMB, on a Docker bind-mount that crosses the VM boundary, or on any shared filesystem that doesn't implement POSIX locking honestly. Local NVMe. Always.

### 2. The "I forgot the WAL files exist" backup failure

`app.db`, `app.db-wal`, `app.db-shm` — three files. If you back up only the first one with a naive `cp`, you'll restore a database that's missing your last batch of writes. Litestream handles this. So does `sqlite3 .backup`. Plain `cp` does not.

### 3. The "long writes block other writes" failure

A 30-second batch INSERT inside a transaction holds the writer lock for 30 seconds. Other writers wait. Break long writes into chunks; commit between chunks. This is the same advice as Postgres, just less hidden.

### 4. The "schema migration without locking" failure

`ALTER TABLE` in SQLite has gotten dramatically better in the last few years (column rename, drop column, etc.) but it still rewrites the whole table for some operations. On a 10GB DB, that's not free. Test the migration on a copy of production first. Same as you'd do with Postgres.

### 5. The "I assumed concurrent writers" failure

There is one writer at a time, ever. WAL gives you concurrent reads, not concurrent writes. If you're running multiple processes that all write to the same DB, you need to funnel writes through one of them, or you need a different database. This is the most common reason people think "SQLite doesn't scale" — they're really running into single-writer.

## When to use Postgres instead

This list is short and important. Use Postgres if:

- **You need multiple writer processes across machines.** Read replicas of an SQLite-backed app are doable but awkward; multi-writer is not.
- **Your hot dataset is going to be over ~100 GB.** SQLite handles big files. The *operational* story for >100 GB SQLite files (backup, restore, point-in-time recovery) gets uncomfortable.
- **You need server-side features that exist only in Postgres.** Full-text search beyond SQLite's FTS5, native JSON paths beyond SQLite's `json1`, listen/notify, logical replication, foreign data wrappers, partitioning.
- **Your team already runs Postgres operationally.** "Boring" is partly a measure of how familiar the operational surface is. If your ops team has a Postgres runbook for everything, picking SQLite forces them to write a new runbook for nothing.

If none of those, SQLite is the smaller choice. Smaller choices are the ones you can change later.

## The migration path

The reason "but what if I outgrow SQLite?" is a smaller worry than people make it: the migration path is genuinely tractable.

```bash
# 1. Install pgloader (one apt-get).
# 2. One command:
pgloader sqlite:///home/app/data/app.db postgresql://user:pass@host/dbname
# 3. Swap your sqlc target from sqlite to postgres in sqlc.yaml.
# 4. Update your connection string.
# 5. Re-run sqlc; build; deploy.
```

Why this works at all: if you've been writing portable SQL through `sqlc` (or just `database/sql`) instead of an ORM, your queries are still queries. Most of them work unchanged on Postgres. The ones that don't are usually `INSERT ... ON CONFLICT` (slightly different syntax) and `AUTOINCREMENT` vs `BIGSERIAL`. Two days of work, not two months.

This is, incidentally, why we picked sqlc over an ORM in the first place: the day you migrate is the day you get paid back for not having an abstraction.

## What this looks like at boringstack.org

The backend for this site runs on SQLite + WAL + Litestream + R2 today. The shape of the operational reality, qualitatively:

- Reads from the HTTP handler are sub-millisecond at p99 — the database is a function call, not a network hop.
- Lock contention has not shown up in the latency graph at any point. Single writer, WAL-enabled, that's the contract.
- The R2 bill for continuous backup sits at zero — well under the free tier with normal write volumes.
- Restore drills run on a cadence and pass. The drill is the backup; the file is just a file.

When any of these stops being true, it'll show up in a future issue with a name attached and the actual numbers on it.

## The deal, again

This is issue #3 of a weekly newsletter. One practical field note every Tuesday — what shipped, what broke, how the bill changed, which defaults held up.

Next Tuesday: **"WAL mode for app builders"** — the deeper read on why WAL changes the shape of what SQLite can be in production.

[Subscribe](/#newsletter) if you don't already. [Star the repo](https://github.com/boringstackoverflow/boring-stack) if you're following along quietly. Reply with your stack — I read every reply.
