# Changelog

All notable changes to the Boring Stack skill + templates land here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project tries to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) — though the "API" here is a Markdown file and a handful of config templates, so the rules bend.

## [0.1.0] — 2026-04-21

Initial public release.

### Added
- `SKILL.md` — opinionated skill definition for Claude Code, plus per-tool variants emitted by `add.sh` for Cursor, Copilot, Codex CLI, Aider, Cline, Continue, Gemini Code Assist, Windsurf, and Zed.
- `MANIFESTO.md` — seven principles for shipping software that stays out of your way.
- `templates/` — production-tested reference configs (`deploy.sh`, `Caddyfile`, `app.service`, `litestream.service`, `litestream.yml`).
- 4-question stack-picker intake that emits `STACK.md` + a `CLAUDE.md` anchor so the chosen stack survives across sessions and resists drift.
- Self-check trigger table that operationalizes the seven manifesto principles as before-output rules (Dockerfile triggers principle 4; deploy script > 30 lines triggers principle 5; new daemon triggers principle 6; etc.).
- Project init recipe (`/boring-stack init` flow) — scaffolds `go.mod`, `main.go` with `/healthz`, `internal/`, `data/`, `deploy/`, `STACK.md`, `CLAUDE.md` anchor, `.gitignore`.
- 8-step post-deploy verification checklist — covers healthz, systemd state, Caddy issued a real Let's Encrypt cert, Litestream wrote a snapshot, replica is reachable, restore drill (`PRAGMA integrity_check ok`), journald is collecting, healthz responds under load.
- `docs/install.sh` — one-line user-level installer (Claude Code symlink, Codex CLI append).
- `docs/add.sh` — one-line project-level installer that auto-detects the project's AI tool configs and drops the right file in each. Supports `--tool <name>` to force, `--tool all` to install everywhere.
- GitHub Pages landing site at `docs/` (`index.html`, `manifesto.html`, `style.css`, `og-image.png`, `favicon.svg`).
- Google Sheets signup endpoint — no backend, no monthly bill.
