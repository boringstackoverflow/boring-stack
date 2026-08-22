---
title: "Boring Stack is now something you can run"
date: 2026-08-22
summary: "Generate a tested Go app and run it locally in one terminal session. The CLI turns the stack's opinions into 19 files that format, test, and build before it reports success."
draft: false
---

Boring Stack started as an AI coding skill.

You installed it, your coding agent received a set of defaults, and the next project was less likely to begin with infrastructure it had not earned.

That was useful, but abstract. You had to take the defaults on faith, because there was nothing to run.

Today Boring Stack becomes something you can see and run:

```bash
curl -fsSL https://boringstack.org/install.sh | bash
boringstack new myapp
cd myapp
boringstack dev
```

<p>
  <a href="https://boringstack.org/?utm_source=newsletter&amp;utm_medium=email&amp;utm_campaign=cli_launch">
    <img src="https://boringstack.org/assets/boringstack-new.gif"
         alt="Terminal recording showing the Boring Stack installer, boringstack new myapp creating 19 files, Go format/test/build checks passing, and the generated app starting on localhost."
         width="960" height="576"
         style="max-width:100%;height:auto;border-radius:6px;border:1px solid #e8e6df;">
  </a>
</p>

<p><em>No jump cuts and no staged output: install Boring Stack, generate a tested Go app, and run it locally in one terminal session.</em></p>

## What the generator actually writes

`boringstack new myapp` creates **19 files**: a small Go app and its test, HTML templates, static CSS, `STACK.md`, `AGENTS.md`, data and migration directories, and the production deploy files — Caddy, systemd, Litestream, and a shell deploy script you can read start to finish.

Then it runs `go fmt`, `go test`, and `go build`. It does not report success until those checks pass.

That last part is the difference worth arguing about. Most generators hand you a directory and wish you luck. This one refuses to claim success for something it has not compiled and tested.

## It is a baseline, not a product

This is not a finished SaaS. It is the production-shaped starting point — the operational decisions already made, the deploy path already written down. From there you write the feature yourself, or you ask your coding agent:

```text
Build me an expense-tracking SaaS using Boring Stack.
```

The CLI and the skill start from the same scaffold. The agent keeps going into the product slice.

## What it needs, plainly

The installer clones or updates Boring Stack and builds the CLI when **Go 1.22+** is available. If Go is missing, it says so, skips the CLI, and still installs the AI skill where supported.

`boringstack doctor --deploy` checks your local deploy prerequisites before you go looking for them at 2am. `boringstack deploy` uses that checked-in shell path, and it needs a target host and a health-check URL — it is not zero-configuration hosting. The host, the domain, the credentials, and the Litestream configuration are still yours to set up.

Boring Stack is MIT licensed and stays that way. Read every file, change the defaults, or tell us where they are wrong — the defaults only improve when people try to break them.

Try it: [boringstack.org](https://boringstack.org/?utm_source=newsletter&utm_medium=email&utm_campaign=cli_launch)

Source: [github.com/boringstackoverflow/boring-stack](https://github.com/boringstackoverflow/boring-stack)
