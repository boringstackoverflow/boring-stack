---
title: "systemd unit files for Go binaries"
date: 2026-07-01
summary: "The boring runtime for a single Go binary: one service file, restart policy, journald logs, graceful shutdown, and enough sandboxing to catch dumb mistakes without inventing a platform."
draft: false
---

A Go binary does not need an orchestrator to stay alive.

It needs a process supervisor. It needs logs. It needs a restart policy. It needs a clear place for config, a clear place for data, and a way to stop without dropping every request on the floor.

On a boring stack, that runtime is `systemd`.

Not because systemd is elegant. It is not. It is a pile of Linux plumbing with a thousand subcommands and a reputation it partly earned. But it is already on the box, it is boring in the best way, and it does the job we actually have: run one binary until we tell it to stop.

The Docker version of this article is longer. That is the problem.

## The boring default

This is the unit file Boring Stack ships for a small Go app:

```ini
[Unit]
Description=app
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=deploy
WorkingDirectory=/home/deploy/app
ExecStart=/home/deploy/app/app
Restart=on-failure
RestartSec=2

KillSignal=SIGTERM
TimeoutStopSec=10s

LimitNOFILE=65535
Environment=PORT=8080

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/deploy/app/data
PrivateTmp=true

ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
RestrictNamespaces=true
LockPersonality=true
SystemCallArchitectures=native

StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Install it at `/etc/systemd/system/app.service`:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now app
sudo systemctl status app
journalctl -u app -f
```

That is the runtime.

No image registry. No compose file. No sidecar. No YAML layer between "start the app" and "the app is running."

## What each line is buying

The first useful line is `User=deploy`. Your app should not run as root. Most small-app disasters do not need a sophisticated attacker. They need one path traversal bug, one bad upload handler, or one shell command with too much permission. Run as the app user.

`WorkingDirectory=/home/deploy/app` gives relative paths a predictable home. Better: make your app use absolute paths for anything important, then the working directory becomes a convenience instead of a dependency.

`ExecStart=/home/deploy/app/app` is the whole launch command. If you cannot explain how your app starts in one line, the deploy has probably grown teeth.

`Restart=on-failure` is the boring babysitter. If the process panics, exits non-zero, or gets killed by a signal that counts as failure, systemd starts it again. It will not fix your bug. It will keep one crash from becoming permanent downtime.

`RestartSec=2` avoids a tight crash loop. Two seconds is enough to keep the box from thrashing and short enough that users probably never notice a single panic.

`KillSignal=SIGTERM` and `TimeoutStopSec=10s` are the graceful shutdown contract. Your Go server should listen for SIGTERM, stop accepting new requests, drain in-flight work, close the database, and exit. If it does not exit within ten seconds, systemd stops waiting.

`StandardOutput=journal` and `StandardError=journal` send logs to the one log store you already have. Read them with:

```bash
journalctl -u app -f
journalctl -u app --since "1 hour ago"
journalctl -u app -p warning..alert
```

That covers most day-one debugging. If your app is small and your logs are useful, `journalctl` is enough for a long time.

## The sandboxing worth turning on

The unit file has a few hardening flags. They are not magic. They are cheap guardrails.

`NoNewPrivileges=true` prevents the process and its children from gaining new privileges through setuid binaries or file capabilities. Your web app should never need that.

`ProtectSystem=strict` makes most of the filesystem read-only from the process's point of view. This catches an entire class of dumb bugs: writing uploads into the app directory, dropping temp files into random places, or letting a bug mutate files it should only read.

`ReadWritePaths=/home/deploy/app/data` punches one deliberate hole in that read-only filesystem. The SQLite file lives there. Uploads can live there if they are local. Nothing else gets write access unless you name it.

`ProtectHome=read-only` keeps the app from writing into home directories by accident. If you need writable app state, give it a specific data directory and put that directory in `ReadWritePaths`.

`PrivateTmp=true` gives the service its own `/tmp`. That removes one more shared scratchpad from the system.

The kernel flags are similar: do not let the web app change kernel tunables, load kernel modules, mess with control groups, create random namespaces, or run binaries for a different architecture. If your app needs any of that, you should be able to explain why in the service file.

## The tempting alternative

The usual alternative is Docker Compose.

For a polyglot app, Compose can be the right choice. If you have a Go API, a Python worker, a Node frontend build server, Redis, and a Postgres container, Compose gives those pieces a shared shape.

For one static Go binary, it mostly adds a container runtime, an image build, a registry or image transfer step, another log command, another restart policy, another network layer, and another file to debug when the app is down.

That can be fine. It is not free.

The boring question is simple: what does the container buy this app today?

If the answer is "reproducible builds," solve that with a pinned Go version and a build script first. If the answer is "isolation," read the hardening flags above again. If the answer is "team standard," fine. Team standard is a real reason. Just write it down.

## What breaks

Three things usually bite small systemd deployments.

First: the app cannot read its config. The service environment is not your shell. If the app needs `DATABASE_PATH`, `PORT`, or `APP_ENV`, put it in the unit file or an `EnvironmentFile=` with locked-down permissions.

Second: the app cannot write the database. That is usually `ProtectSystem=strict` doing its job. Fix the path, not the policy. Put the database under `/home/deploy/app/data`, make the directory owned by the service user, and name it in `ReadWritePaths`.

Third: restart loops hide the first failure. `systemctl status app` shows you the current state. `journalctl -u app --since "10 minutes ago"` shows you the story. Read the first error, not the tenth restart.

## When this stops being enough

Use something heavier when the shape changes.

If you have multiple services with conflicting runtimes, Compose can be useful. If you have many teams deploying independently, an orchestrator starts making sense. If you need rolling deploys across a fleet, a single service file is no longer the whole runtime.

But do not borrow that complexity on day one.

The migration path is boring: keep the Go binary, keep the HTTP port, keep Caddy in front, and move the process boundary later. A systemd service can become a container. A container can move behind a load balancer. Nothing about the first version prevents the second.

## This week's receipt

The repo now ships the actual template at [`templates/app.service`](https://github.com/boringstackoverflow/boring-stack/blob/main/templates/app.service). It is not a gist. It is part of the stack contract.

If the template is wrong, the project is wrong. Open an issue. Send the flag that bit you. The goal is not to worship systemd; the goal is to have a service file a tired builder can read at 2am and still make the right change.

## The deal, again

This is issue #6. Next up: the single Go binary deploy — build, copy, swap, restart, smoke test. The whole deploy path should fit on a postcard, and every extra line has to earn rent.

Boring is not old. Boring is the part of the system you can explain when the page goes off.

[Star the repo](https://github.com/boringstackoverflow/boring-stack). Try the [`app.service`](https://github.com/boringstackoverflow/boring-stack/blob/main/templates/app.service) template on a small app. Reply with the line you had to change first.
