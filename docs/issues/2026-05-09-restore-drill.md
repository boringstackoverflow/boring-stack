---
title: "Restore drill #1: when the WAL bites"
date: 2026-05-09
summary: "Borela's first scheduled restore drill found a 1.2-second data loss window. One Litestream config line closed it."
draft: false
---

Borela's first scheduled restore drill ran on Friday at 02:00 UTC.
The SQLite WAL had 14 unflushed frames at restore time, and Litestream's
checkpoint loop hadn't fired yet — so the restored DB was missing the
last 1.2 seconds of writes.

The fix was a one-line config tweak: `max-checkpoint-page-count: 100`,
down from the default 1000. Trades a tiny throughput hit for a tighter
recovery window.

## Numbers from the actual drill

| Metric                  | Value           |
| ----------------------- | --------------- |
| Restore time            | 4.7s for 12 MB  |
| Frames replayed         | 14              |
| Loss window before fix  | ~1.2s           |
| Loss window after fix   | 0 frames at p99 |

## What the skill argued for, and what we actually shipped

The boring-stack skill says "SQLite + WAL + Litestream is the default."
That's still true. What this drill showed is that the *defaults* matter:
out of the box, Litestream replicates with checkpoints sized for
throughput, not for recovery point objective. If you care about RPO
(most apps don't, until the drill), the page-count knob is the lever.

For the curious, the diff is in the
[Borela config repo](https://github.com/boringstackoverflow/borela-infra)
under `litestream.yml`.

## Next week

- [ ] Restore drill #2 with simulated agent crash mid-write
- [ ] First customer install (Borela has a real beta user now — more soon)
- [ ] Skill change: tighter pushback wording when projects ask for "Postgres for vector search" and only need an FTS5 index

If you've run a restore drill on your own boring-stack project, reply with
your numbers — I'd like to publish a comparison next month.
