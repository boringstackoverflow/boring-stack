# Boring Stack

A Claude Code skill plus reference templates for building web apps the boring way.

I'm building Borela — a babysitter service for web apps running on the boring stack (pre-launch, no customers yet, no public site). Its full cloud control plane (marketing, magic-link auth, dashboard, locked v1 protocol, dev mailer) runs for under $8 a month: one Go binary, one SQLite file, Litestream replicating to Cloudflare R2, Caddy fronting it, systemd running it, a $5 Hetzner VPS hosting it.

This repo is the part that makes Claude do the same by default.

## What's in the box

- **`/boring-stack`**: a Claude Code skill that argues for the boring stack when you start a side project. Pushes back on Postgres, Vercel, ORMs, Docker, Kubernetes, microservices when they don't fit. Two sentences plus a question per pushback. Always defers to your call.
- **`templates/`**: battle-tested reference configs from the Borela production stack. `deploy.sh` (10 lines), `Caddyfile` (8 lines), `app.service` (hardened systemd unit), `litestream.service`, `litestream.yml`. Drop them in, replace the hostnames, ship.
- **`MANIFESTO.md`**: seven principles. Quote them, link them, fork them.

## Who this is for

Side-project-first builders. Indie hackers. Solo founders. Anyone whose project profile fits: small data, single region, low write contention, a real human can be paged when something breaks.

## Who this is NOT for

If any of these match your project, use a different stack. The skill will say so on your behalf.

- More than ~100GB of database
- Multi-region active-active
- Compliance where downtime matters in human-life terms (medical, life-safety)
- More than ~50 engineers on the codebase
- Real-time collaboration on shared state

`SKILL.md` has the full "this is not for you if" section.

## Live landing page

[boringstackoverflow.github.io/boring-stack](https://boringstackoverflow.github.io/boring-stack/) — manifesto, install snippet, signup form. Hosted on GitHub Pages straight out of `docs/`. The signup form posts to a Google Apps Script that appends rows to a Google Sheet — no backend, no monthly bill.

## Install

One line. Works with every major AI coding tool.

```bash
curl -fsSL https://boringstackoverflow.github.io/boring-stack/install.sh | bash
```

That clones the skill to `~/.boring-stack` and wires it into the tools that have a user-level config (Claude Code, Codex CLI). Idempotent — re-run any time to update.

For tools that only support project-level rules (Cursor, Copilot, Cline, Aider, Gemini, Windsurf, Continue, Zed), run this from inside any project where you want the boring stack defaults:

```bash
cd /path/to/your/project
curl -fsSL https://boringstackoverflow.github.io/boring-stack/add.sh | bash
```

Auto-detects which tools the project uses and drops the right file in each. Falls back to `AGENTS.md` (the portable convention) if nothing's set up yet.

### What gets written, per tool

| Tool | File written |
|---|---|
| Claude Code | `~/.claude/skills/boring-stack/SKILL.md` (user) or `.claude/skills/boring-stack/SKILL.md` (project) |
| Codex CLI (OpenAI) | `~/.codex/instructions.md` (appended) |
| Cursor | `.cursor/rules/boring-stack.mdc` |
| GitHub Copilot | `.github/copilot-instructions.md` (appended) |
| Cline | `.clinerules` |
| Continue.dev | `.continuerules` |
| Aider | `CONVENTIONS.md` (appended) |
| Gemini Code Assist | `GEMINI.md` (appended) |
| Windsurf | `.windsurfrules` |
| Zed | `.rules` |
| Anything else | `AGENTS.md` (portable fallback) |

### Force a specific tool

If auto-detect picks the wrong thing (or you want to install for a tool whose config files don't exist yet):

```bash
curl -fsSL https://boringstackoverflow.github.io/boring-stack/add.sh | bash -s -- --tool cursor
curl -fsSL https://boringstackoverflow.github.io/boring-stack/add.sh | bash -s -- --tool all
```

Recognized values: `claude`, `cursor`, `copilot`, `codex`, `aider`, `cline`, `continue`, `gemini`, `windsurf`, `zed`, `agents`, `all`.

### Use it

In Claude Code: type `/boring-stack`. In Codex CLI / Cursor / Copilot / etc.: the rules load automatically when the tool reads its config.

## Use the templates without the skill

The configs in `templates/` work standalone. Copy the file, replace the hostname / domain / R2 account placeholders, ship.

```bash
# example: bring up a new VPS
scp templates/Caddyfile root@your-vps:/etc/caddy/Caddyfile
scp templates/app.service root@your-vps:/etc/systemd/system/app.service
scp templates/litestream.service root@your-vps:/etc/systemd/system/litestream.service
scp templates/litestream.yml root@your-vps:/etc/litestream.yml
ssh root@your-vps "systemctl daemon-reload && systemctl enable --now caddy app litestream"
```

## License

MIT. See `LICENSE`.

## Why this exists

Borela is being built as a babysitter for web apps running on the boring stack. When it ships, it'll verify your backups are restorable, drill the restore on a schedule, watch the agent heartbeat, and page someone when the cron silently dies. Planned pricing is $5/mo for backup verification, $9/mo for the full babysitter. **Pre-launch — no customers yet, no signups open, no public site.** This OSS skill is the funnel that ships first.

If you install the skill and start running the boring stack on your side projects, some of those projects will grow up and need a babysitter. That's the bet.

The skill is MIT and stays MIT no matter what happens to paid Borela. Use it, fork it, ignore it, write a better one.

## Contributing

Issues and PRs welcome. The bar for new pushback rules is high: each one needs a real trade-off with numbers, a real migration path, and an honest "want it anyway?" closer. The skill loses credibility if it pushes the boring stack at projects that don't fit, so the "When NOT to use this skill" section in `SKILL.md` is load-bearing too.

If you've got a war story (boring stack saved you, boring stack failed you, you migrated off and here's why), open an issue. The monthly notes from the build pull from those.

## Maintained by

[boringstackoverflow](https://github.com/boringstackoverflow). Reach me at boringstackoverflow@gmail.com.
