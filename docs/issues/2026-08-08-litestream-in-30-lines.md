---
title: "Litestream in 30 lines"
date: 2026-08-08
summary: "The backup layer for a single SQLite file: one config file, one systemd unit, continuous replication to Cloudflare R2, and a restore command you should run before you need it. Eleven lines of actual configuration."
draft: false
---

A SQLite database is a file. That is the best thing about it and the scariest thing about it.

The best thing, because backing up a file is a solved problem. The scariest thing, because "it's just a file" is exactly what people say right before they copy it while it's being written to and get a corrupted snapshot they don't discover for six weeks.

The boring answer is Litestream. It is a separate process that watches your SQLite WAL and streams frames to object storage, continuously, while the app runs. Your app does not change. Your code does not import anything. You do not add a backup endpoint or a cron job that shells out to `sqlite3 .backup`.

You add one config file and one service file.

## The boring default

This is `templates/litestream.yml`, shipped in the repo:

```yaml
dbs:
  - path: /home/deploy/app/data/app.db

    snapshot-interval: 1h
    retention: 168h

    replicas:
      - type: s3
        endpoint: https://<account>.r2.cloudflarestorage.com
        bucket: app-backups
        path: app.db
        access-key-id: ${R2_KEY}
        secret-access-key: ${R2_SECRET}
```

That is eleven lines of actual configuration. The file in the repo is thirty-three, and the other twenty-two are comments explaining the two numbers you might want to change.

The service file is the same shape as the app's:

```ini
[Service]
Type=simple
ExecStart=/usr/bin/litestream replicate -config /etc/litestream.yml
Restart=always
RestartSec=5

KillSignal=SIGTERM
TimeoutStopSec=10s
LimitNOFILE=4096

EnvironmentFile=/etc/litestream.env
```

Install both, then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now litestream
journalctl -u litestream -f
```

Your database now has an off-box copy that is seconds old.

## What the two numbers buy

`snapshot-interval: 1h` is how often Litestream writes a full snapshot instead of another WAL frame. The default is 24h. Snapshots make restores faster, because a restore replays every frame since the last snapshot — a 24-hour gap means replaying up to 24 hours of writes. They also cost storage, because each snapshot is a full copy of the database.

An hour is the middle. If your database is 50 MB and mostly idle, 24h is fine and cheaper. If it takes writes all day, an hour keeps restore time short enough that you don't dread it.

`retention: 168h` is seven days. After that, older snapshots and frames get pruned. The default is 24h, which is enough to survive a hardware failure and not enough to survive a bad migration you didn't notice until Thursday. Seven days gives you room to roll back from your own mistakes, which are more common than dead disks.

Those two lines are the whole tuning surface. That is the point.

## Restore is the feature

Replication is not the feature. Restore is the feature. Replication is the tax you pay to have one.

```bash
litestream restore -config /etc/litestream.yml \
  -o /tmp/restored.db /home/deploy/app/data/app.db

sqlite3 /tmp/restored.db "pragma integrity_check;"
sqlite3 /tmp/restored.db "select count(*) from your_busiest_table;"
```

Three commands. The first pulls the database down. The second asks SQLite whether the file is coherent. The third asks you whether the data is actually there, which `integrity_check` will never tell you — a perfectly intact database with zero rows passes integrity checks with a cheerful `ok`.

Run this on a laptop. Run it against production storage. It costs nothing, because R2 does not charge for egress, which is the single most underrated line on the bill we published in [issue #5](https://boringstack.org/issues/2026-05-30-the-cloud-bill-is-an-architecture-review.html). The reason most backup setups quietly rot is that testing them costs money or attention. When the test is free and fits on one screen, you run it.

If you have never run that command against your own backups, you do not have backups. You have a configuration file that expresses an intention.

## The honest note

This site's own backend does not run Litestream.

We wrote that down in issue #5 and it is still true. `boringstack.org`'s API shares a VPS with `borela.dev`, and `borela-agent` was already snapshotting to R2 on that box, so adding a second project block to a running agent beat standing up a parallel service. The bill bends to what is already on the machine.

The OSS skill still defaults to Litestream for fresh projects, and the templates above are the ones it writes. But you should know which parts of this newsletter are field reports and which parts are the recommended default, and this one is the recommended default. When those two things diverge, we will keep saying so.

## What breaks

**The database is not in WAL mode.** Litestream replicates WAL frames. No WAL, nothing to replicate. `pragma journal_mode=wal;` once, at startup, forever.

**Two Litestream processes on one database.** Don't. Litestream assumes it owns the replication position for a given database. Running a second one against the same file — a leftover from testing, a copy of the unit file with a different name — produces a replica that looks healthy and restores wrong.

**Secrets in the config file.** `${R2_KEY}` reads from the environment, and `EnvironmentFile=/etc/litestream.env` is mode 0600, root-only. If you paste your R2 keys directly into `litestream.yml`, they are now in whatever backup captured that config, and probably in a git repo.

**Silence that looks like success.** `systemctl status litestream` says `active (running)` whether it is replicating or failing every attempt against a bucket that no longer exists. `journalctl -u litestream --since "1 hour ago"` is the actual health check. Better: restore, on a schedule.

## When this stops being enough

Litestream is disaster recovery, not high availability. It gives you a recent copy somewhere else. It does not give you failover, and it does not give you read replicas you can serve traffic from.

If you need a warm standby that takes over in seconds, you have outgrown this — look at litestream's `live read replica` mode, or accept that you are in Postgres territory now. If your restore window is longer than your business can survive, the fix is not a bigger `retention` value; it is a second machine.

The migration path stays boring. Keep the SQLite file. Add the second replica — the template ships a commented-out Backblaze B2 block for cross-provider redundancy, because "all my backups are at one vendor" is a real failure mode. Then, if you actually need it, move to a database with replication built in. Nothing about this setup blocks that.

## This week's receipt

The repo ships both files: [`templates/litestream.yml`](https://github.com/boringstackoverflow/boring-stack/blob/main/templates/litestream.yml) and [`templates/litestream.service`](https://github.com/boringstackoverflow/boring-stack/blob/main/templates/litestream.service). Same contract as `app.service` — if the template is wrong, the project is wrong, so open an issue.

## The deal, again

This is issue #7, and it is late. Issue #5 promised a restore drill "next Tuesday." That was ten weeks ago. Issue #6 promised the deploy script and then went quiet for five.

So: no promise this week, just a debt. The restore drill is the next issue. Pulling a real database out of R2 onto a fresh box, with a stopwatch, and publishing the number even if the number is embarrassing. It has been scheduled twice and skipped twice, and a newsletter about backups that will not test its own backups is just content.

Boring is not effortless. Boring is *the small set of things you can still do correctly when you are tired.*

[Star the repo](https://github.com/boringstackoverflow/boring-stack). Run the restore command against your own backups tonight. Reply with how long it took — I will publish the range, including mine.
