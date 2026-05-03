---
title: "Why this exists"
date: 2026-05-02
summary: "Most projects do not need a complicated stack on day one. Boring Stack is the public default — philosophy, AI coding skill, templates, and weekly notes."
draft: false
---

Most projects do not need a complicated stack on day one.

They need a way to ship, learn, and keep the system understandable while the product is still earning the right to be complex.

Boring Stack exists for that stage.

The default is intentionally plain:

- one VPS
- one Go binary
- one SQLite database
- Litestream backups
- Caddy for HTTPS and reverse proxying
- systemd for process management
- shell scripts for deploys
- logs you can read with `journalctl`
- restore drills you actually run

This is not about pretending infrastructure is the hard part of every startup. It usually is not. Most projects fail because they do not find enough demand.

That is exactly why the stack should stay boring early on.

When you are still figuring out whether anyone wants the thing, your infrastructure should be cheap, legible, recoverable, and easy to change.

## Boring does not mean unserious

Boring Stack is not nostalgia for old tools.

It is a preference for tools with small surfaces and clear failure modes.

Go builds into one binary.

SQLite keeps the database close to the app.

Litestream gives that database a continuous backup story.

Caddy handles HTTPS without making certificate renewal a project.

systemd runs the process, restarts it when it fails, and gives you logs in one place.

A VPS is not glamorous, but it is understandable. You can SSH into it. You can inspect the files. You can see what is running. You can move the app to another box.

That matters.

Boring Stack is not "never use Postgres," "never use Docker," or "never use the cloud."

It is:

> Start with the smallest production-shaped system you can understand, operate, back up, and replace.

Add complexity when the project has earned it.

## Why this site exists

This site is the public home for that default.

It has three jobs.

### 1. Explain the stack

Every part of the stack should be understandable.

Not just "use SQLite," but when SQLite is enough, what WAL mode changes, where the database file should live, how backups work, and when to move to Postgres.

Not just "use systemd," but what a unit file does, where logs go, how restarts work, and how to run workers or multiple apps.

Not just "use a VPS," but how to provision it, secure it, deploy to it, back it up, monitor it, and recover on a fresh box.

The goal is not to make every builder a full-time sysadmin.

The goal is to make the system small enough that the builder can understand the important parts.

### 2. Publish the templates

The repo includes the practical pieces:

- `deploy.sh`
- `Caddyfile`
- `app.service`
- `litestream.service`
- `litestream.yml`
- agent rules
- project conventions

These templates are not meant to be magic.

They are meant to be readable.

A good deploy script should invite editing. A good service file should be understandable. A good backup setup should have a restore path you have tested.

If the app stops, you should know where the logs are.

If the database disappears, you should know where the replica is.

If the deploy fails, you should be able to read the script and see what happened.

That is the bar.

### 3. Keep the defaults honest

Simple does not mean sloppy.

A one-VPS app still needs:

- backups
- restore drills
- deploy checks
- health checks
- logs
- basic metrics
- alerts
- security updates
- rollback paths
- cost visibility

The newsletter is where that work becomes public.

Each week, I want to publish one useful field note: what shipped, what broke, what the bill says, what the restore drill found, which default held up, and which default needs changing.

Some issues will be tutorials. Some will be cost receipts. Some will be template notes. Some will be mistakes.

The loop is simple:

> publish the defaults, test the defaults, show the receipts, improve the defaults.

## Where Borela fits

[Borela](https://borela.dev/) is the production proof point behind the stack.

It runs on this shape of infrastructure and exists to watch the boring parts: backups, restore drills, heartbeats, and jobs that silently stop.

But Boring Stack is not just a wrapper around Borela.

Boring Stack is the public layer:

- the philosophy
- the AI coding skill
- the templates
- the defaults
- the examples
- the weekly notes
- the mistakes
- the receipts

Some projects may eventually want a babysitter.

Most will not.

That is fine.

If Boring Stack helps a builder ship a small app with a deploy path they understand and a backup they have actually restored, it worked.

## This is not a new bet

I am not pretending this idea is original.

[Pieter Levels](https://levels.io) has been writing about this shape of stack for years.

[Steve Hanov](https://stevehanov.ca/blog/how-i-run-multiple-10k-mrr-companies-on-a-20month-tech-stack) has shown that meaningful revenue can run on modest infrastructure.

DHH has spent a career arguing against complexity that has not earned its way in.

The broader indie web, self-hosting, Rails, Go, SQLite, Linux, and small-tools communities have all been circling the same point:

> Most software can start simpler than it does.

What Boring Stack adds is a packaged default for the current builder workflow.

The skill gives coding agents a different starting point.

When an agent reaches for Postgres, it can ask whether SQLite fits.

When it reaches for Vercel, it can suggest Caddy on a VPS.

When it reaches for Docker or Kubernetes, it can ask whether systemd and one binary are enough.

When it reaches for an ORM, it can suggest `sqlc` and plain SQL.

Not every pushback will be right.

That is fine.

The goal is not to win architecture debates. The goal is to make complexity a conscious choice.

## Try it

Install the skill. Read the templates. Argue with the defaults.

Use Boring Stack if it fits.

Ignore it when it does not.

The point is simple:

> Ship software you can still understand six months from now.
