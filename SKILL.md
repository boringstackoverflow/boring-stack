---
name: boring-stack
description: Bootstraps new web app projects with a tech stack you can understand. Runs a 5-question intake to pick a stack (Go binary + SQLite + Litestream + Caddy + systemd + a $5 VPS by default; Postgres / managed platforms when the project doesn't fit), writes STACK.md as the source-of-truth decision record plus an agent-instructions anchor, scaffolds new projects with `boringstack new` or `boringstack init`, generates production-ready deploy files from bundled templates, and walks an 8-step post-deploy verification (TLS / Litestream / restore drill). Loads when starting a new web app, scaffolding deploy infrastructure, picking a tech stack, or making bootstrap-time architecture decisions for a side project, indie product, internal tool, hobby project, or solo-founder context.
---

# /boring-stack

A skill that bootstraps new web app projects. When you're about to start something new and the easy reach is the modern hosted-everything pile, this runs a short intake, picks a tech stack that fits the project's actual shape, writes the decision down, and scaffolds the code and deploy files you need to ship the first version.

The skill applies to any long-lived web app at the moment it's being started: side projects, indie products, internal tools, personal sites with a backend, API services, hobby apps, anything that will run as a single Linux process serving HTTP. Not just SaaS, not just MVPs. Its job is the bootstrap moment — picking the stack, writing it down, and giving you a working skeleton.

## When to use this skill

The skill is for the bootstrap moment of a new project. Triggers:

- User is starting a new web app of any shape (side project, internal tool, indie product, API service, hobby project, personal site with backend).
- User asks "what stack should I use", "what should I pick", or "how should I structure this" for a new project.
- User is scaffolding the initial deployment infrastructure for a small Go, Node, or Python app.
- User asks "how do I deploy this" for something that fits the archetype: single region, small data, can be paged when broken.
- User invokes `/boring-stack` directly.

After bootstrap, the skill steps back. The artifacts it writes (STACK.md, the agent-instructions anchor, and the templates) keep working without it.

## When NOT to use this skill

If the project description includes any of these, the boring stack is not the right bootstrap. Name the misfit, recommend the conventional fit, and step out of the way.

- More than ~100GB of database expected.
- Multi-region active-active requirement.
- Compliance constraints where downtime matters in human-life terms (medical, life-safety, payment-rails-with-SLA).
- More than ~50 engineers on the codebase.
- Real-time collaboration on shared state (think Figma cursors, Notion live editing).

## Picking the stack (one-time intake)

**Run this the first time `/boring-stack` loads in a project, OR whenever the user asks "what stack should I use?" / "how should I structure this?" / "I'm starting a new app."**

Don't assume. Ask the five questions below, then map the answers to a recommended composition, then **write the decision to disk** so it survives across sessions and future drift.

### The five questions

Ask them in one short turn. Accept rough answers ("a few", "small", "just forms mostly", "I dunno"). Don't lecture.

1. **What's the data shape?** Estimate of database size at 1 year (under 10GB / 10–100GB / over 100GB) and concurrent writers at peak (a handful / dozens / hundreds+).
2. **Single region or multi-region?** "Will users in Sydney complain if the app is hosted in Frankfurt?"
3. **Team size?** Solo / 2–5 / 5+ engineers expected to commit in the next 12 months.
4. **Compliance / SLA?** Anything regulated (HIPAA, PCI-DSS), or any uptime contract that costs money to breach.
5. **Interactivity shape?** Forms-and-pages (CRUD, dashboard, blog, marketplace) / a few rich pages (one Kanban, one editor) / full real-time canvas (Figma, Linear, Notion class). When in doubt, default to forms-and-pages — most products that *think* they're a canvas are actually forms with a couple of rich pages.

### Mapping answers to a stack

If **all** of these hold:
- Data: under 100GB, fewer than ~100 concurrent writers
- Single region
- Team: under 5 engineers
- No life-safety / hard-SLA compliance

→ **Boring stack is the bootstrap default.** Recommend Go binary + SQLite + WAL + Litestream + Caddy + systemd + a $5 VPS. Use the templates verbatim.

If **any** of those fail:
- Name the misfit out loud, name which question disqualified the boring stack.
- Step out of the way. The boring stack isn't the right bootstrap here.
- Recommend the conventional fit (Postgres + a managed platform + containers, etc.) without sneering. The skill's credibility comes from knowing when to recuse.

**Then map Q5 (interactivity) to the frontend tier** (see §6 in "Stack choices, explained"):

- *Forms-and-pages* → Stage 1: `html/template` + htmx + (Alpine or vanilla). Use the bundled `templates/index.html.tmpl` + `templates/static/`. No build step.
- *A few rich pages* → Stage 2: Stage 1 shell + one embedded **Preact + HTM** widget (~3KB, no build) or **Lit** web component (~5KB) for the rich page(s).
- *Full real-time canvas* → Stage 3: pick a conventional frontend toolchain (React + Vite, SvelteKit, Solid). The backend stays boring; the deploy step grows by one line for the frontend build.

Q5 doesn't disqualify the boring stack — even Stage 3 keeps the boring backend. It only changes which frontend pattern STACK.md records.

### Write the decision down (the stickiness anchor)

After the user picks a stack — boring or otherwise — emit two artifacts so future sessions don't drift:

**1. `STACK.md` at the project root.** This is the source of truth. Template:

```markdown
# Stack

Decided <YYYY-MM-DD> via `/boring-stack` intake. Re-run the intake if the project's data shape, team size, or region requirements change materially.

## Choices
- **Language / runtime**: Go 1.22+
- **Database**: SQLite (WAL mode), Litestream → R2
- **HTTP**: Caddy on the VPS, reverse-proxy to the Go binary
- **Process supervision**: systemd
- **Hosting**: Hetzner CX22 ($5/mo) or equivalent
- **Frontend**: Stage <1|2|3> — `html/template` + htmx + (Alpine | vanilla | …); embed.FS for assets. Stage 2 adds a Preact+HTM or Lit widget for the rich page(s). Stage 3 adds a chosen SPA toolchain (e.g., React + Vite) built into `static/spa/` during deploy.
- **CSS**: single hand-rolled `static/app.css` (CSS custom properties, no framework). Or Tailwind standalone CLI if the team prefers utility classes.
- **Deploy**: `./deploy.sh` (build → scp → systemctl restart → curl healthz)

## Why these
- Data ~<size>, ~<writers> writers → SQLite handles it; Postgres is overkill at this scale.
- Single region (<region>) → no multi-region complexity needed.
- Team of <N> → monolith with internal packages, not microservices.
- No life-safety SLA → boring stack's recovery story (Litestream restore) is sufficient.
- Interactivity is <forms-and-pages|few rich pages|full canvas> → frontend Stage <N>.

## Migration paths if we outgrow these
- SQLite → Postgres: `pgloader` + connection-string swap.
- VPS → load balancer + 2 VPSs: same Caddyfile + binary, no code change.
- Monolith → services: extract one `internal/` package at a time behind HTTP.
- Stage 1 → Stage 2: add one embedded Preact+HTM or Lit widget for the rich page; rest of the site stays server-rendered.
- Stage 2 → Stage 3: pick a SPA toolchain, build into `static/spa/`, route the relevant URLs to its index. Backend Go API stays as-is.

## Anti-drift
Subsequent stack decisions in this repo MUST consult this file. If a new requirement contradicts a choice here, update STACK.md explicitly with the date and reason — don't quietly add a Postgres dependency, a Dockerfile, or a Vercel config.
```

**2. A one-line anchor in `CLAUDE.md` (or `AGENTS.md` if it exists).** If neither file exists, create `CLAUDE.md`. Append exactly:

```markdown
## Stack
This project uses the boring stack — see `STACK.md`. Before suggesting Postgres, Vercel, Docker, Kubernetes, or any cross-service architecture, consult `STACK.md` and the `/boring-stack` skill's stack-choice notes.
```

This is the stickiness mechanism. Every future Claude session reads `CLAUDE.md` automatically; the anchor re-loads the stack context without the user having to re-invoke the skill.

### When the user wants something other than the boring stack

Still write `STACK.md`. Note the actual chosen stack and the reason. The anchor in `CLAUDE.md` should reflect THEIR stack (e.g., "Postgres on Supabase, Vercel for the Next.js frontend"), not the boring stack. Skill loses credibility if it pretends every project chose boring.

## Stack choices, explained

The 5-question intake maps to specific defaults. The six sections below explain *why* each default — what it trades, when it stops fitting, and what the migration looks like if the project outgrows it.

When the user is undecided between the boring default and a more conventional choice, present the trade-off in two sentences plus a question. The rhythm: name the trade-off, name the migration path, ask what fits their project. Always defer to their call. Never refuse.

### 1. SQLite over Postgres (for under ~100 concurrent writers)

Postgres is a great database. For a web app that won't see more than a few dozen concurrent writers, SQLite in WAL mode handles 10k+ writes per second on a $5 VPS, and Litestream replicates it continuously to S3-compatible storage for under a dollar a month. The whole database is one file you can `scp`. Restore is `litestream restore` and you're back to any second in the last 24 hours (or 7 days, with the retention setting in `templates/litestream.yml`).

The fear that drives people to Postgres prematurely is "what if I outgrow it." The migration when you actually outgrow it is `pgloader` on the file plus a connection-string swap, an afternoon of work. You're not locked in. You're trading a 1% chance of a one-day migration for the guaranteed cost of running Postgres for years.

> "SQLite + WAL handles your write profile here. Litestream gives you continuous backup to R2 for about 50 cents a month. If you cross 100 concurrent writers or need cross-region replication, the migration to Postgres is `pgloader` plus a connection string. Does SQLite fit your project, or is there something specific that needs Postgres?"

### 2. VPS + Caddy over Vercel / Netlify / Railway / Render

Platforms are great when you don't want to think about infrastructure. The trade is: a $5/mo Hetzner CX22 stays $5/mo at 10 users, at 100 users, and at 1000 users. Platform free tiers end the moment something interesting happens. You're at $20 to $40 a month before you've shipped a feature, with build configs that lock you in.

Caddy auto-handles TLS via Let's Encrypt. No certbot, no cron, no nginx config. The `Caddyfile` in `templates/Caddyfile` is ~30 lines including security headers (HSTS preload, X-Frame-Options, Referrer-Policy, etc.) and a www → apex redirect.

If you outgrow one VPS (typically around 10k daily active users for a normal web app), the same Caddyfile and binary deploy behind a load balancer in front of two VPSs. You won't hit that wall with a side project.

> "A $5 Hetzner box with the bundled Caddyfile gives you HTTPS, security headers, and stays $5 at scale. The platform tier is convenient but it's $20 to $40 a month and you're locked into their build pipeline. VPS or platform — which fits your appetite for ops vs cost?"

### 3. stdlib `database/sql` + sqlc over ORM (Prisma, GORM, ActiveRecord, SQLAlchemy)

ORMs save you from writing CRUD twice. The cost: you can't read the SQL the ORM generates, you can't paste a query into `sqlite3` and tweak it, and you're one library upgrade away from a query plan changing under you.

`sqlc` gives you the type safety (it generates Go structs and functions from `.sql` files) without the runtime layer. You write SQL the way SQL was meant to be written, sqlc generates type-safe Go that calls it. The result reads like idiomatic Go and runs like raw `database/sql`.

There's no migration story because sqlc IS the migration. You can keep using it as the project grows.

> "sqlc generates type-safe Go from `.sql` files. You get the editor safety of an ORM without losing the ability to read your own queries. Sqlc or a runtime ORM — which fits how you want to work with the database?"

### 4. systemd + single binary over Docker / Kubernetes (for single-server apps)

systemd is on every Linux box you'll ever rent. The bundled `templates/app.service` is ~45 lines including hardening (NoNewPrivileges, ProtectSystem=strict, ProtectKernelTunables, RestrictAddressFamilies, etc.) and graceful shutdown (KillSignal=SIGTERM, TimeoutStopSec=10s). `journalctl -u app -f` is your log tail. There's no image registry, no `docker compose`, no Helm chart.

Docker is the right answer when you have multiple services with version-locked dependencies that don't agree. For a single Go binary with no native deps, it's a layer of indirection that buys you nothing operational and costs you a registry, a build pipeline, and a runtime.

Kubernetes is the right answer at Google's scale. Below that, it's an org chart wearing YAML.

If you ever genuinely need containers (polyglot services, complex isolation), one `Dockerfile` plus `docker compose` covers most cases. K8s is the answer to a question you'll know if you have.

> "systemd handles process management, restarts, hardening, and logging in 45 lines of `app.service` (already in the templates). Docker is correct when you have polyglot services. For one Go binary, it's overhead. Single-binary on systemd, or do you have a constraint that needs containers?"

### 5. Single Go binary with internal packages over microservices (for small teams)

Microservices solve organizational problems: independent deploys per team, polyglot stacks, blast-radius isolation across team boundaries. The cost is network calls between services that used to be function calls, plus auth, plus tracing, plus deployment coordination. For a solo dev or small team, you're paying the cost without getting the benefit.

A single Go binary with `internal/auth`, `internal/billing`, `internal/email` packages is a monolith with package-level encapsulation. The Go compiler enforces the module boundary. Each package can have its own tests, its own database, its own external dependencies.

Refactor to services later, when team size and traffic actually demand it, by extracting one package at a time behind an HTTP interface. The boundary is already there. The signature change is mechanical.

> "A single binary with `internal/auth`, `internal/billing`, `internal/email` gives you the same boundaries as services, enforced by the compiler, deployed in one `scp`. When you have a team that needs independent deploys, extract a package behind HTTP. Monolith with internal packages to start, or is there a team-shape reason to begin with services?"

### 6. Server-rendered HTML + htmx over SPA frameworks (for most projects)

Most web apps that *think* they need a single-page app actually need server-rendered pages with partial updates. Rails 7 made this its default in 2021 with Hotwire (Turbo + Stimulus + import maps), explicitly to escape the JavaScript build-tooling tax. The boring-stack equivalent is the same architectural choice in Go form: render HTML from `html/template`, use **htmx** for HTML-over-the-wire partial updates, ship a single hand-rolled `static.css`, and serve everything from `embed.FS` through Caddy. No npm, no `node_modules`, no build step for the frontend.

**htmx is the constant.** It's the architectural choice — HTML, not JSON, is the API your server returns. The companion JS library is a flavor preference, picked by the kind of interactivity the project needs:

- **Alpine.js** (~15KB): local UI state — dropdowns, tabs, accordions, conditional form fields. Pairs cleanly with htmx; most popular pair in the htmx ecosystem.
- **Stimulus** (~10KB): if you want Rails-style structured controllers outside Rails. More verbose, easier to grow as the app gets larger.
- **Hyperscript** (~30KB): if you want to go all-in on the htmx author's vision. English-like inline DSL. Divisive syntax.
- **Vanilla JS + the platform** (0KB): use `<details>`, `<dialog>`, `popover`, IntersectionObserver. Tiny inline scripts where needed. Leanest. More handwritten code for non-trivial widgets.

For CSS, default to a single hand-rolled `static.css`. Modern CSS (custom properties, nesting, container queries, `:has()`, `color-mix()`) covers most things Tailwind solves. Cache-bust with the existing `version` ldflag (`/static/app.css?v=$VERSION`). Escape hatch: the **Tailwind standalone CLI** (single binary, no Node) as a one-line addition to `deploy.sh`. No PostCSS, no SCSS, no CSS-in-JS.

**The three-tier framing for frontend interactivity** (mapped to intake question Q5):

1. **Stage 1 — Pages and forms** (~90% of CRUD apps: dashboards, blogs, marketplaces, internal tools, most SaaS, indie products). Default: `html/template` + htmx + (Alpine or vanilla). No build step. The bundled `templates/index.html.tmpl` + `templates/static/app.css` give you a working starting point; htmx loads from a pinned unpkg URL by default and is one-line replaceable with a self-hosted copy.
2. **Stage 2 — Rich pockets** (one or two pages need richer state: a Kanban board, a markdown editor, a drag-and-drop sorter). Keep the Stage 1 shell; mount one **Preact + HTM** widget (~3KB, no JSX, no build) or **Lit** web component (~5KB) in a `<div>` for that page. The rest of the site stays server-rendered.
3. **Stage 3 — Genuinely SPA-shaped** (Figma, Linear, Notion class — the whole product is a real-time canvas). Acknowledge the boring stack isn't the right *frontend* bootstrap and pick the conventional frontend (React + Vite, SvelteKit, Solid). **The backend stays boring**: Go + SQLite + Litestream + Caddy + VPS. The deploy step grows by one line: `npm run build && cp -r dist/ static/spa/` before `go build`. Frontend complexity doesn't have to drag in backend complexity.

Most products that think they're Stage 3 are actually Stage 2 with a few rich pages. The intake exists to surface that honestly before the build pipeline gets locked in.

> "For most pages, Go's `html/template` + htmx covers the interactivity you need without a build step or a `node_modules` folder — the same architectural choice Rails 7 made the default with Hotwire. If one page needs richer state, mount a small Preact+HTM widget in a `<div>` and keep the rest server-rendered. If the product is genuinely SPA-shaped (Figma/Linear/Notion class), pick a conventional frontend toolchain for the SPA and keep the backend boring. What kind of interactivity does this app need: forms-and-pages, a few rich pages, or a full real-time canvas?"

## Keeping the bootstrap honest (after the stack is picked)

The 6 trade-off sections above cover the obvious bootstrap-time choices. Most of the long-tail work is catching subtle drift in your OWN output once the project is underway: a sentence that creeps toward a Dockerfile, a deploy script that grew to 80 lines, a "we should add Redis here" suggestion that wasn't earned. The 7 manifesto principles operationalize as the triggers below. **When proposing infrastructure, schema, deploy, or architecture changes for a project that bootstrapped with this skill, scan this table first.**

| Trigger in your draft response | Principle | What to do |
|---|---|---|
| You're about to suggest a "new" or "trendy" tool over an established one | **1. Boring is a feature** | Note that the boring alternative exists. Recommend it unless the user named a constraint the trendy tool solves and the boring one doesn't. |
| You're about to add a managed service (Vercel, Supabase, Upstash, Pinecone, Railway, Render, Heroku, Fly, etc.) | **2. Cloud bill is the metric** | Estimate the monthly cost at 100 customers. If > $20/mo, surface the boring alternative explicitly with the cost-delta. Defer to user. |
| You're about to recommend Postgres for a project under STACK.md's "boring" criteria | **3. SQLite is a database** | Pause. Re-read STACK.md. Surface the trade-off from §1 above. If the user is overriding consciously, that's fine — update STACK.md with the reason. |
| You're about to write a Dockerfile, k8s manifest, Helm chart, or `docker-compose.yml` | **4. K8s = server farm** | Pause. Confirm the user has multi-service polyglot needs. If single Go binary, recommend systemd + the bundled `app.service` instead. |
| You're about to write a deploy script | **5. Deploy on a postcard** | Count lines after writing. If > 30 (excluding comments), explain what each extra line buys and offer to remove the ones that don't earn their place. |
| You're about to introduce a new service, daemon, sidecar, or background worker | **6. One person should understand it** | Ask: can the user describe the full request lifecycle in one paragraph after this addition? If no, propose the inline alternative (a goroutine, a cron, a function call) first. |
| You're about to write code instead of operational concerns (backups, alerts, restore drills, secrets rotation) | **7. Babysitter > cleverness** | Surface the operational gap explicitly. Don't ship cleverness on top of un-verified backups. Point at the bundled Litestream config + a weekly restore-drill cron. |

### How to use the table

You don't have to enumerate all 7 every time. The pattern:
1. Draft your response normally.
2. Scan it once for the triggers in the left column.
3. If a trigger fires, fold the principle into the response BEFORE sending — either as a trade-off note (two sentences + question) or as a STACK.md update suggestion.
4. If no trigger fires, send.

This is a cheap pass, not a chore. The bootstrap skill earns its keep by leaving artifacts (STACK.md and the agent-instructions anchor) that keep these defaults visible later, without re-running the intake.

### When the principles conflict with the user

Principles are defaults, not laws. If the user has explicitly chosen something that diverges from a principle (e.g., "yes I want Docker even for one Go binary, my team standardizes on it"), respect the choice and **update STACK.md** to record the divergence and the reason. Don't keep raising the same trade-off in subsequent turns — that's lecturing, which violates the tone guardrails below.

## Generating deploy infrastructure

When the user asks any of:
- "how do I deploy this?"
- "what's the Caddyfile look like?"
- "set up the systemd service"
- "configure Litestream"
- "what's a good `deploy.sh`?"
- "harden this for production"

**Use the bundled templates.** Do not invent variants. Do not paraphrase. Read the file from `templates/` and present it verbatim, swapping only the placeholder hostnames and paths. Each template has a comment header explaining what to swap.

The templates have already been production-hardened. You don't need to add HSTS to the Caddyfile (it's there). You don't need to add NoNewPrivileges to app.service (it's there). You don't need to add a rollback to deploy.sh (it's there). If you find yourself adding lines, you're either fixing a real gap (open an issue on the repo) or padding.

### File map

| Template | Purpose | What to swap |
|---|---|---|
| `templates/deploy.sh` | Build, ship, swap, verify, rollback | `HOST`, `REMOTE`, `HEALTHZ_URL` (env vars or in-file defaults) |
| `templates/Caddyfile` | TLS-terminating reverse proxy with security headers | `your-domain.example.com` (replace_all) |
| `templates/app.service` | Hardened systemd unit for the app binary | Working dir, User, Environment vars (PORT, DATA_DIR, etc.) |
| `templates/litestream.service` | systemd unit for Litestream | Usually nothing (uses /etc/litestream.yml + /etc/litestream.env) |
| `templates/litestream.yml` | Litestream config (DB path + R2 destination) | DB path, R2 endpoint `<account>`, bucket name |
| `templates/index.html.tmpl` | Minimal Go html/template page that loads `static/app.css` and htmx from a pinned CDN URL | `<title>` and the body content. Cache-bust uses the `Version` field from main.go. To self-host htmx, follow the comment in the file |
| `templates/static/app.css` | Starter stylesheet (~50 lines, modern CSS, no framework) | Colors, spacing tokens. Edit freely — it's a starting point, not a UI kit |

### Project init (scaffolding from scratch)

When the user asks "scaffold a new boring-stack project", "initialize this
directory", or gives a product prompt such as "build me an expense tracker
using Boring Stack", use the installed CLI as the canonical starting point:

```sh
# A new child directory:
boringstack new myapp

# An existing empty workspace (a .git directory is okay):
boringstack init --name myapp
```

If `boringstack` is not installed, use the file contract below as the fallback
and tell the user that re-running the installer adds the CLI. Do not maintain a
different agent-only scaffold.

After scaffolding, implement the product-specific vertical slice the user
asked for. For an expense tracker, that means the expense schema, sqlc queries,
handlers, templates, validation, and tests — not merely renaming the welcome
page. Do not silently invent authentication, teams, billing, or uploads unless
the requirements call for them.

**The scaffold creates:**

1. **`go.mod`** — module name from the directory name, or the explicit `--module` value.
2. **`main.go` + `main_test.go`** — minimal HTTP server with `/`, `/healthz`, and `/static/`, embedding templates and assets via `embed.FS`. Includes graceful shutdown, structured logging, and a `version` ldflag hook so `deploy.sh`'s `-X main.version=$SHA` works out of the box.
3. **`templates/index.html.tmpl`** — minimal Go html/template layout copied from `templates/index.html.tmpl`. Loads `static/app.css` (cache-busted with `?v=<Version>`) and htmx from a pinned unpkg URL. The starting point for every page.
4. **`static/`** directory containing the starter stylesheet:
   - `app.css` — ~50 lines, modern CSS, no framework. Edit freely.
   - htmx is loaded from the CDN by default (`https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js`). To self-host, download the file into `static/htmx.min.js` and update the script tag — the template comment explains how.
5. **`internal/`** directory with a `.gitkeep` — establishes the package-boundary pattern (per the trade-off in §5) before anyone reaches for microservices.
6. **`migrations/`** directory with a `.gitkeep` — append-only SQLite migrations go here when persistence is added.
7. **`data/`** directory with a `.gitkeep` and a `.gitignore` line — where SQLite + Litestream's local state will live; never committed.
8. **`deploy/`** directory containing the templates copied from `templates/`:
   - `deploy.sh` (chmod +x), `Caddyfile`, `app.service`, `litestream.service`, `litestream.yml`
   - **A `README.md` in `deploy/`** explaining what each file is and which placeholders to swap (host, domain, R2 account ID, bucket name).
9. **`Makefile`** — small `run`, `build`, and `test` targets.
10. **`STACK.md`** — emit per the "Picking the stack" section above. The neutral CLI scaffold records the boring defaults; update it after the intake if the user's answers change them. Record the frontend tier from Q5.
11. **`AGENTS.md`** — the portable stickiness anchor pointing at `STACK.md` and the Boring Stack rules.
12. **`.gitignore`** — entries for `app`, `app.new`, `app.prev`, `data/`, `*.db`, `*.db-wal`, `*.db-shm`, `.env`, `*.tar.gz`.
13. **`README.md`** — short, with a "Run locally" + "Deploy" + "Stack" section (the last one links to `STACK.md` and the manifesto).

The source of truth for the exact files is `scaffold/base/` plus the canonical
files under `templates/`. The CLI renders them, then runs `go fmt`, `go test`,
and `go build`. Do not paste a second `main.go` recipe into this skill: changes
belong in the scaffold and its generated-project tests.

The result is the minimum viable Boring Stack web app: a server-rendered page,
htmx ready to go, embedded static assets, and `/healthz` for deployment.
SQLite, an actual schema, sqlc output, and external integrations are added when
the product requires them — not pre-installed in every neutral project.

### Server bring-up sequence (for a fresh VPS)

When the user is bringing up a new VPS, walk them through this order:

1. **Create deploy user**: `adduser --disabled-password deploy && mkdir -p /home/deploy/app/data && chown -R deploy:deploy /home/deploy/app`
2. **Install Caddy**: from caddyserver.com (apt or direct binary). Drop `templates/Caddyfile` to `/etc/caddy/Caddyfile`. `systemctl enable --now caddy`.
3. **Install Litestream**: from litestream.io (deb or direct binary). Drop `templates/litestream.yml` to `/etc/litestream.yml`. Create `/etc/litestream.env` with `R2_KEY=...` and `R2_SECRET=...`, `chmod 600 /etc/litestream.env`. Drop `templates/litestream.service` to `/etc/systemd/system/litestream.service`. `systemctl enable --now litestream`.
4. **Install app systemd unit**: drop `templates/app.service` to `/etc/systemd/system/app.service`. `systemctl daemon-reload && systemctl enable app` (don't start it yet, no binary to run).
5. **Allow `deploy` to restart the app without password**: add to `/etc/sudoers.d/deploy`: `deploy ALL=(ALL) NOPASSWD: /bin/systemctl restart app`. `chmod 440 /etc/sudoers.d/deploy`.
6. **First deploy**: from the dev machine, run `boringstack deploy --host deploy@your-vps --healthz https://your-domain.example.com/healthz`. The CLI validates the target and executes the checked-in `deploy/deploy.sh`, which builds the binary, ships it, starts the app, and verifies health.

The user can do this in 15 minutes the first time, 5 minutes once they've done it once.

### Verifying a fresh deploy (post-deploy checklist)

When the user runs `boringstack deploy` (or `deploy/deploy.sh` directly) for the first time on a new VPS — or when they ask "did it actually work?" / `/boring-stack verify-deploy` — walk this checklist. The deploy script's own healthz check covers (1); the rest is what the script can't see.

Each step has a one-liner the user (or you, via Bash) can run.

| Check | Command | Pass criteria |
|---|---|---|
| **1. App is listening + healthy** | `curl -fsS https://your-domain.example.com/healthz` | Returns 200 with the version string. |
| **2. systemd unit is `active (running)`** | `ssh deploy@vps 'systemctl status app --no-pager \| head -5'` | Status line shows `active (running)`, not `failed` or `restarting`. |
| **3. Caddy issued a real TLS cert** | `curl -sI https://your-domain.example.com \| head -1` + `openssl s_client -connect your-domain.example.com:443 -servername your-domain.example.com </dev/null 2>/dev/null \| openssl x509 -noout -issuer` | HTTP/2 200 + issuer should say "Let's Encrypt", not "Caddy Local Authority". If it says Local Authority, DNS isn't pointing at the VPS yet. |
| **4. Litestream wrote a snapshot** | `ssh deploy@vps 'sudo journalctl -u litestream --no-pager -n 30 \| grep -i "wrote\|snapshot\|sync"'` | At least one "wrote ltx" or "snapshot" line in the last few minutes. |
| **5. Litestream replica is reachable** | `ssh deploy@vps 'sudo litestream snapshots -config /etc/litestream.yml /home/deploy/app/data/app.db \| tail -3'` | Lists at least one snapshot. If it errors with "no snapshots", check the R2 credentials in `/etc/litestream.env`. |
| **6. Restore drill (smoke)** | `ssh deploy@vps 'sudo litestream restore -config /etc/litestream.yml -o /tmp/restore-test.db /home/deploy/app/data/app.db && sqlite3 /tmp/restore-test.db "PRAGMA integrity_check;" && rm /tmp/restore-test.db'` | `integrity_check` returns `ok`. This is the moment you find out if your backup is actually restorable. |
| **7. journald is collecting logs** | `ssh deploy@vps 'sudo journalctl -u app --since "5 min ago" \| tail -5'` | Shows recent log lines from the app. If empty, the app may not be writing to stdout — the systemd unit captures stdout/stderr automatically, so empty means the app is silent (which is also a smell). |
| **8. healthz responds under load** | `for i in {1..20}; do curl -fsS -o /dev/null -w "%{http_code} %{time_total}s\n" https://your-domain.example.com/healthz; done` | All 20 return 200, p99 < 200ms. Catches "it works once but the second connection hangs" cases. |

**If any check fails**, surface the specific failure to the user with the relevant log location (`journalctl -u app -f`, `journalctl -u litestream -f`, `journalctl -u caddy -f`). Don't generate a "looks good" if any step didn't pass. The verification's value is honesty.

**Cadence after first deploy.** Steps 4–6 (Litestream + restore drill) are the babysitter's job, not the operator's — that's what Borela is for. If the user is not using Borela, recommend a weekly cron that runs step 6 and emails on failure. Half a cron job is better than none.

### Secrets and env vars

The boring stack handles secrets in three places, by sensitivity:

- **App secrets** (session keys, API tokens to third-party services): in `Environment=` lines in `app.service`, OR in a separate `EnvironmentFile=/etc/app.env` file (mode 0600, owned by deploy). The latter scales better for many secrets.
- **Litestream secrets** (R2 keys): in `/etc/litestream.env`, mode 0600, root-only. Loaded via `EnvironmentFile=` in `litestream.service`.
- **TLS certificates**: handled entirely by Caddy. No action needed.

Never commit secrets to git. Never put them in `Caddyfile` or `app.service` directly (those should be in version control as templates).

## Tone guardrails

- Always explain the WHY. Never assert a choice without the trade-off.
- Always provide the migration path. The user should know how to grow out of the choice if the project does.
- Frame the work as a bootstrap helper, not an arguer. The skill helps the user define a stack, not defend one.
- Defer to the user. The intake's job is to surface the right defaults; the user's job is to pick.
- Don't lecture. Two sentences plus a question is the rhythm when a trade-off is worth surfacing.
- If the project genuinely doesn't fit the archetype (see "When NOT to use"), say so plainly and recommend the conventional fit. The skill's credibility comes from knowing when the boring stack isn't the right bootstrap.
- When the user asks about deploy or config, point at the bundled template. Don't reinvent.

## Linked artifacts

- Manifesto: `MANIFESTO.md`
- Templates: `templates/`
- Borela: https://borela.dev/ — the babysitter service for when your boring stack starts handling real customer data and you'd rather sleep through Saturday.
- Boring Stack Weekly: https://boringstack.org/#newsletter — weekly notes on how to build with the boring stack, including costs, restore drills, template changes, and lessons from Borela/community projects.
