---
title: "Your SQLite database belongs on the VPS"
date: 2026-11-07
summary: "Keep the live database on persistent local disk. Back it up off-site, preserve the WAL files, and prove you can recover when the entire VPS disappears."
draft: false
---

If your Go app runs on one VPS, keep its live SQLite database on that VPS too.

Give it a persistent directory. Keep that directory out of your binary replacement step. Replicate changes to storage outside the VPS, and practice bringing the application back on another machine.

That is a reasonable production architecture for a small app. The part that needs care is everything hidden inside the word *persistent*.

## One local database, one off-site recovery path

The application reads and writes the local database. Litestream, or a service built around it, continuously copies changes to an off-site replica.

```text
VPS                             Off-site storage
Go app → local SQLite → agent → snapshots and changes
                                      ↓
                               isolated restore drill
```

The object store is a backup destination. The app does not fetch a SQLite file from R2 for each request or mount a bucket as its production database.

SQLite's [WAL documentation](https://www.sqlite.org/wal.html) requires processes sharing a WAL database to run on the same host. A network filesystem shared between application servers is not a way to turn this setup into a distributed database.

For one process on one machine, local storage keeps the ordinary request path small. When you need writers on several hosts or automatic failover, revisit the architecture against that requirement.

## Put data outside the release you replace

Either of these can work:

```text
/home/deploy/app/data/app.db
/var/lib/myapp/app.db
```

The first is the current Boring Stack reference path. The second follows a common Linux service-data convention. Moving a healthy database just to change the spelling of its directory buys little.

What matters is that your deploy replaces the executable without replacing the data directory. Our [binary deploy script](2026-08-12-the-single-go-binary-deploy.html) already keeps those operations separate.

In a container, persist the whole SQLite directory with a volume or bind mount. Putting only `app.db` on persistent storage leaves the surrounding files in a different lifetime from the database that needs them.

Keep the directory out of Git, release cleanup and temporary storage. Leave room for the database, its journal, replication state and maintenance operations. A 200 MB database does not mean 200 MB of disk is enough.

## WAL changes what “one file” means

With WAL enabled, committed writes can be in `app.db-wal` before a checkpoint incorporates them into `app.db`. SQLite also uses `app.db-shm` for coordination.

Those files belong to the running database. Do not delete them to tidy the directory, and do not copy a live `app.db` with `scp` and assume you captured a current, coherent backup. Copying the surrounding files one at a time is not an atomic backup either.

For a one-off live snapshot, use SQLite's [online backup API](https://www.sqlite.org/backup.html), exposed by the CLI's `.backup` command. Run this as a user with the required access, after creating a protected destination directory:

```sh
sqlite3 -bail -readonly /var/lib/myapp/app.db \
  ".backup '/var/backups/myapp/app-backup.db'"

sqlite3 -bail -readonly /var/backups/myapp/app-backup.db \
  'PRAGMA integrity_check;'
```

Use a new destination for a snapshot you need to retain. Expect the integrity result to be `ok`; an automated check must inspect that result rather than treating the command's exit status as proof of integrity. This is a consistent local snapshot, and still shares the VPS's failure domain until you copy it elsewhere.

For continuous off-site protection, use Litestream's supported replication and restore path for your installed version. Test that version's actual configuration and commands instead of mixing examples from different releases.

## Configure the connections you actually open

For an app adding SQLite, these are useful starting settings:

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = FULL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
```

WAL mode persists for the database. The other settings shown here belong to the connection, so configure every connection your Go driver or pool opens. Running the statements once in a separate `sqlite3` session does not configure your application's pool.

[`synchronous=FULL`](https://www.sqlite.org/pragma.html#pragma_synchronous) is a conservative durability choice. In WAL mode, `NORMAL` reduces synchronization work but can lose recently committed transactions after a power failure. Choose that tradeoff deliberately. A five-second busy timeout bounds waiting for locks; it does not create additional concurrent writers.

Test these settings with your workload. Customer count alone says little about write contention: a hundred mostly idle accounts and one busy import job can behave very differently.

## The backup user needs working permissions

Run the application without root privileges. Give the backup process a dedicated identity and only the access its implementation requires.

That includes traversing every parent directory, accessing the database and sidecars, and creating or updating replication state. Litestream can need write access for its database bookkeeping and checkpointing; a read grant on `app.db` alone is not a complete permission recipe.

Use an app-specific group or ACLs, including inheritance for newly created files. Verify replication as the actual service user after an app restart, when sidecars may have been recreated. Keep unrelated application users excluded and credentials out of world-readable configuration.

The [permission-checking lesson from issue #10](2026-09-01-your-backup-agent-cant-read-your-database.html) still applies: checking as root does not tell you whether the backup user can do the job.

## Choose frequency from the loss you can tolerate

Backup frequency describes several different things:

| Operation | What you choose it for |
| --- | --- |
| Incremental replication | How much recent committed data might be missing after host loss |
| Full snapshots | The amount of history a restore needs to replay |
| Retention | How far back you can recover from a mistake noticed later |
| Restore drills | How soon you discover that recovery has stopped working |

Write down two targets: the data-loss window you can accept, called RPO, and how long the product can be unavailable, called RTO. Then measure replication lag and a recovery exercise against them. A configured one-second interval is not a promise that every commit is safe off-site within one second.

Daily snapshots with continuous incremental replication can be a starting point; a database with heavy writes or a short recovery deadline may need a different policy. Test restore time, retained history and storage/API usage before tuning. Small databases can still generate enough object-store requests to make requests a meaningful part of the bill.

## Borela automates the recurring checks

[Borela](https://borela.dev/) is the service we build for this operating model: a local SQLite database on a VPS, continuous off-site backups, and evidence that a restore works.

It adds scheduled isolated restores, integrity checks, backup-freshness monitoring, failure notifications and a dashboard of results. You can use it for the recurring backup checks while keeping the application and its database on your VPS. The standalone alternative is Litestream plus scheduled restore checks and an alert path you have tested.

A successful database drill is useful evidence. Bringing the whole product back also requires the application binary, configuration, secrets, routing and a working host. Keep a secure recovery copy of those requirements outside the VPS, and practice using it.

## Practice losing the whole machine

On a disposable replacement host:

1. Retrieve your recovery instructions and credentials without depending on the original VPS.
2. Install the tested application and backup-tool versions. Restore into a fresh path with the app stopped.
3. Check integrity, the expected schema and known application data. An empty or stale database can be internally valid.
4. Apply the service ownership and permissions, start the app, and perform a real application read.
5. Confirm replication resumes. Record elapsed recovery time and the newest committed data recovered.

For a planned cutover, stop writes to the old instance before switching traffic. Once the replacement accepts writes, falling back to the old database requires preserving and reconciling those writes.

Also keep an independent public uptime check, disk-space alerts and an OS patch/reboot routine. The primary VPS cannot notify you while it is gone.

We have opened [the production-defaults and recovery-runbook work in the OSS repo](https://github.com/boringstackoverflow/boring-stack/issues/13). The recommendations here are the target; that issue tracks the template and verification changes still to do. This article does not claim a measured replacement-host recovery time.

The useful question for tonight: **if your VPS disappeared, could you recover the app without reading a single file from it?**
