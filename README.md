# Boring Stack

A project generator, AI coding skill, reference templates, and builder movement for making web apps the boring way.

[Borela](https://borela.dev/) is live: a babysitter service for web apps running on the boring stack. Its full cloud control plane (marketing, magic-link auth, dashboard, locked v1 protocol, dev mailer) runs for under $8 a month: one Go binary, one SQLite file, Litestream replicating to Cloudflare R2, Caddy fronting it, systemd running it, a $5 Hetzner VPS hosting it.

This repo is the open-source part that makes AI coding tools do the same by default. The project is also becoming a public record of how to build this way: weekly notes, real costs, restore drills, launch lessons, and the trade-offs behind every boring choice.

## What's in the box

- **`boringstack new myapp`**: a small Go CLI that creates and verifies a runnable web app, including tests, stack decisions, and the production deploy templates. `dev`, `doctor`, and `deploy` keep the path from local page to prepared VPS concrete.
- **`/boring-stack`**: an AI coding skill that bootstraps a new app with the same scaffold. Runs a 5-question intake, picks a stack that fits, writes the decision to `STACK.md`, and turns product prompts into a complete vertical slice. Surfaces trade-offs (SQLite vs Postgres, VPS vs Vercel, systemd vs Docker, monolith vs microservices) as two-sentence-plus-question notes. Always defers to your call.
- **`templates/`**: battle-tested reference configs from real production use. `deploy.sh` (10 lines), `Caddyfile` (8 lines), `app.service` (hardened systemd unit), `litestream.service`, `litestream.yml`. Drop them in, replace the hostnames, ship.
- **`MANIFESTO.md`**: seven principles. Quote them, link them, fork them.
- **The newsletter**: weekly field notes on building with Boring Stack: what shipped, what broke, what the bill says, which defaults held up, and which ones need to be sharpened.

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

## Live landing page and newsletter

[boringstack.org](https://boringstack.org/) — CLI quickstart, manifesto, newsletter signup, and OSS launch page. Hosted on GitHub Pages straight out of `docs/`; newsletter signup posts to the small Go service in the companion `boring-stack-backend` repository.

The newsletter is weekly and practical: one build log, one decision, one operational lesson. It is for people using Boring Stack on side projects, indie products, internal tools, and small web apps that should stay legible.

## Install

One line. Works with every major AI coding tool.

```bash
curl -fsSL https://boringstack.org/install.sh | bash
```

That clones the project to `~/.boring-stack`, builds the CLI into `~/.local/bin/boringstack` when Go 1.22+ is available, and wires the skill into tools that have a user-level config (Claude Code, Codex CLI). Idempotent — re-run any time to update.

Create something concrete:

```bash
boringstack new myapp
cd myapp
boringstack dev
```

The generator refuses to overwrite existing work, then formats, tests, and builds the result. For an agent-built product, ask: `Build me an expense-tracking SaaS using Boring Stack.`

For tools that only support project-level rules (Cursor, Copilot, Cline, Aider, Gemini, Windsurf, Continue, Zed), run this from inside any project where you want the boring stack defaults:

```bash
cd /path/to/your/project
curl -fsSL https://boringstack.org/add.sh | bash
```

Auto-detects which tools the project uses and drops the right file in each. Falls back to `AGENTS.md` (the portable convention) if nothing's set up yet.

### What gets written, per tool

| Tool | File written |
|---|---|
| Boring Stack CLI | `~/.local/bin/boringstack` |
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
curl -fsSL https://boringstack.org/add.sh | bash -s -- --tool cursor
curl -fsSL https://boringstack.org/add.sh | bash -s -- --tool all
```

Recognized values: `claude`, `cursor`, `copilot`, `codex`, `aider`, `cline`, `continue`, `gemini`, `windsurf`, `zed`, `agents`, `all`.

### Use it

From a terminal, run `boringstack new myapp`. In Claude Code, type `/boring-stack`. In Codex CLI / Cursor / Copilot / etc., the rules load automatically when the tool reads its config.

## Anonymous usage reporting

The installer and the `boringstack` CLI report a few anonymous events to
`api.boringstack.org` so we can see whether installs actually succeed and
whether generated projects actually build:

| When | Event | What is sent |
|---|---|---|
| `install.sh` finishes | `install_success` | install ID, OS/arch, git revision, whether the CLI built |
| `add.sh` finishes | `install_project` | install ID, OS/arch, number of tool configs written |
| `boringstack new`/`init` | `project_created`, `project_validated` | install ID, OS/arch, CLI version, `new` vs `init`, whether fmt/test/build passed |

The install ID is a random string stored at `~/.boring-stack/.install-id`. It is
not derived from anything about you or your machine. Delete the file and you
become a new install.

**Never sent:** your name, email, IP-derived identifiers, project names, module
paths, directory paths, git remotes, or any file contents.

Reporting is best-effort and capped at two seconds. It cannot fail an install,
delay a scaffold beyond half a second, or change any command's exit code — an
unreachable endpoint is simply skipped. The installer prints a line when it
reports. Read the code: [`docs/install.sh`](docs/install.sh),
[`docs/add.sh`](docs/add.sh),
[`cmd/boringstack/telemetry.go`](cmd/boringstack/telemetry.go).

There is currently no opt-out flag. If that matters to you, the events are three
`curl` calls and one Go file — delete them and rebuild, or install by cloning
the repo directly instead of running `install.sh`.

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

[Borela](https://borela.dev/) has launched. It is the babysitter for web apps running on the boring stack: it verifies your backups are restorable, drills the restore on a schedule, watches the agent heartbeat, and pages someone when the cron silently dies.

This OSS project is the public layer around that bet. Borela proves the stack in production; Boring Stack gives builders the skill, templates, and defaults to build the same way before they need a babysitter.

The newsletter is the movement loop: weekly notes on what shipped, what broke, what the bill says, what the restore drill found, and which boring defaults held up in real apps. Some projects will grow into Borela customers. Others will only need the principles, templates, and receipts. Both outcomes make the movement stronger.

The skill is MIT and stays MIT no matter what happens to commercial Borela. Use it, fork it, ignore it, write a better one.

## Contributing

Issues and PRs welcome. The bar for new stack-choice trade-offs is high: each one needs real numbers, a real migration path, and an honest "does this fit your project?" closer. The skill loses credibility if it bootstraps the boring stack at projects that don't fit, so the "When NOT to use this skill" section in `SKILL.md` is load-bearing too.

If you've got a war story (boring stack saved you, boring stack failed you, you migrated off and here's why), open an issue. The weekly newsletter pulls from those reports and from real projects using the stack.

## Maintained by

[@boringstack](https://x.com/boringstack). Reach me at hello@boringstack.org.
