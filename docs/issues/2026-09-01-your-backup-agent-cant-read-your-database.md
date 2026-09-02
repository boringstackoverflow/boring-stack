---
title: "Your backup agent can't read your database"
date: 2026-09-01
summary: "A backup that runs on schedule, reports success, and copies nothing is the most expensive kind. Unix path traversal and SQLite's WAL file each produce one, and neither raises an error. Here is how to check yours in one command."
draft: false
---

The worst backup is not the one that fails. It is the one that runs on schedule, exits zero, logs nothing unusual, and copies nothing.

Two things produce that outcome on a boring stack, and neither one raises an error. The first is a directory your backup user cannot walk through. The second is a SQLite file that holds almost none of your data.

Both are checkable in a single command. Most people never run it, because the obvious way to check hides the problem.

## The boring default

Run the backup agent as its own user, and give it read access to the database with a POSIX ACL:

```sh
setfacl -m u:borela:rx /home/bsb/app/data
setfacl -m u:borela:r  /home/bsb/app/data/app.db
```

That looks complete. It grants read on the directory and read on the file, and `setfacl` exits zero for both.

It can still back up nothing.

## Why it silently fails

`useradd -r bsb` creates `/home/bsb` as mode `0750`, owned by `bsb:bsb`. A user that is neither `bsb` nor a member of group `bsb` has no execute bit on that directory, and on Unix the execute bit on a directory is permission to *traverse* it — to resolve a name inside it at all.

So the ACL on `/home/bsb/app/data` is correct, and unreachable. The kernel refuses at `/home/bsb`, three levels up, before the grant you wrote is ever consulted.

The reason this survives code review is the failure mode. The agent does not get far enough to open a file, so it has no file error to report. There is no permission denied in the log, because nothing was denied at the point where the code checks. The unit runs. The timer fires. The dashboard is green.

This exact bug shipped in the Boring Stack backend's own `setup.sh` and sat there until August 16. The database had never once been readable by the backup agent. The script granted the ACL and asserted it had worked, which is not the same as finding out.

## The second silent failure: WAL

SQLite in WAL mode does not write committed transactions into the main database file right away. They accumulate in `app.db-wal` until a checkpoint folds them in.

An agent that has read access to `app.db` and not to `app.db-wal` therefore backs up a file that is real, valid, openable — and arbitrarily stale. It might be missing an hour of writes. It might be missing everything since the last checkpoint, which on a low-traffic app can be a long time.

This one is worse than the traversal bug, because it produces an artifact. You get a backup file. It passes an integrity check. It is simply not your database.

Grant all three:

```sh
for f in app.db app.db-wal app.db-shm; do
  setfacl -m u:borela:r "/home/bsb/app/data/$f"
done
```

## The trade-off

ACLs are the smallest tool that solves this. They keep the app's home directory closed to everyone else and open one narrow path for one user.

The cost is that they are invisible. `ls -l` shows a `+` at the end of the mode string and nothing else; you need `getfacl` to see what is actually granted. A teammate reading the directory listing will not know the rule exists, and a restore onto a fresh box will not have it unless the setup script applies it. That is a real argument for the alternative: put the backup user in a group, give the group traversal, and let `ls -l` tell the whole story.

Use the group when more than one process needs the path. Use the ACL when exactly one does, as here — `x` for traversal, never `rx`, because the backup agent has no business listing the contents of your home directory.

## Copy/paste: grant, then verify

The grant is the part everyone writes. The verification is the part that matters:

```sh
# Traversal on every directory in the path. x only, not rx.
setfacl -m u:borela:x  /home/bsb
setfacl -m u:borela:x  /home/bsb/app
setfacl -m u:borela:rx /home/bsb/app/data

# Future files inherit read.
setfacl -d -m u:borela:r /home/bsb/app/data

# The WAL and shared-memory files, not just the database.
for f in app.db app.db-wal app.db-shm; do
  [ -f "/home/bsb/app/data/$f" ] && setfacl -m u:borela:r "/home/bsb/app/data/$f"
done

# Verify as the actual user. This is the whole point.
if sudo -u borela test -r /home/bsb/app/data/app.db; then
  echo "✓ borela can read app.db"
else
  echo "✗ borela cannot read app.db — backups will NOT work"
  sudo -u borela ls -l /home/bsb/app/data/
fi
```

Put those last five lines in your setup script, not in your head. A grant that is never tested is a comment.

## The mistake that hides all of this

Checking the path as root.

Root ignores the permission bits. `ls -l /home/bsb/app/data/app.db` as root prints the file, confirms it exists, shows a sane mode, and tells you nothing whatsoever about whether `borela` can open it. Every debugging session that begins with `sudo ls` ends with "looks fine to me."

The check has to run as the user that will do the work:

```sh
sudo -u borela test -r /home/bsb/app/data/app.db
```

Same shape as the general rule. Do not verify a backup by looking at the backup job. Verify it by being the thing that reads.

## Checking what came back

Once the file is genuinely readable and genuinely current, the restore side has its own version of the same trap. SQLite's own integrity test:

```sh
sqlite3 app.db "PRAGMA integrity_check;"
```

returns:

```text
ok
```

That proves the file is internally coherent. It does not prove it contains your data — an empty database returns `ok`, and so does a three-week-old one.

Check the contents against something you know:

- the applied schema-migration version
- row counts on the tables that should never be empty
- the newest timestamp in your busiest table, compared against production
- one real application read, through the actual server, against the restored file

The last one is the only check that exercises the same code path your users do.

## Failure mode

Everything green, nothing backed up. No alert fires, because nothing errored. You discover it the first time you need the backup, which is the worst possible moment to discover anything.

The tell, if you go looking: a backup destination whose object size never changes, or changes far less than your database does.

## Outgrow path

When the app grows past one process reading one file — a second service that needs the same database, a read replica, a separate reporting job — stop patching ACLs onto a private home directory. Move the data out to a shared location with a real group (`/srv/app/data`, group `app-data`, mode `2750`), and let the setgid bit keep new files inside the group.

The verification step does not change. It is still `sudo -u <the user> test -r <the file>`, and it still belongs in the script.

## The one thing to take away

Every backup system has a step where you assert a permission and a step where you use it. If those are the same step, you do not have a backup — you have an intention.

Run the check on your own box now. It takes ten seconds:

```sh
sudo -u YOUR_BACKUP_USER test -r /path/to/your.db && echo ok || echo BROKEN
```
