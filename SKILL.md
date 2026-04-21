---
name: boring-stack
description: Opinionated defaults for building web apps the boring way. Go binary, SQLite + Litestream, Caddy, systemd, a $5 VPS. Runs a 4-question intake to pick a stack, writes STACK.md + a CLAUDE.md anchor so the choice survives across sessions, scaffolds new projects from scratch (`init`), generates production-ready deploy files from bundled templates, walks an 8-step post-deploy verification (TLS / Litestream / restore drill), and pushes back when Claude reflexes toward Postgres, Vercel, Docker, Kubernetes, or microservices for projects that don't need them. Loads when starting a new web app, scaffolding deploy infrastructure, or making stack decisions for a side project, indie product, internal tool, hobby project, or solo-founder context.
---

# /boring-stack

A skill that argues. When you're starting a web app and the easy reach is the modern hosted-everything pile, this loads a different set of defaults and pushes back when the conversation drifts.

The skill applies to any long-lived web app: side projects, indie products, internal tools, personal sites with a backend, API services, hobby apps, anything that runs as a single Linux process serving HTTP. Not just SaaS, not just MVPs.

## When to use this skill

- User is starting a new web app of any shape (side project, internal tool, indie product, API service, hobby project, personal site with backend).
- User is scaffolding deployment infrastructure for a small Go, Node, or Python app.
- User asks "what stack should I use" or "how do I deploy this" for something that fits the archetype: single region, small data, can be paged when broken.
- User invokes `/boring-stack` directly.

## When NOT to use this skill

If the project description includes any of these, do not push the boring stack. Note the misfit and step out of the way.

- More than ~100GB of database expected.
- Multi-region active-active requirement.
- Compliance constraints where downtime matters in human-life terms (medical, life-safety, payment-rails-with-SLA).
- More than ~50 engineers on the codebase.
- Real-time collaboration on shared state (think Figma cursors, Notion live editing).

## Picking the stack (one-time intake)

**Run this the first time `/boring-stack` loads in a project, OR whenever the user asks "what stack should I use?" / "how should I structure this?" / "I'm starting a new app."**

Don't assume. Ask the four questions below, then map the answers to a recommended composition, then **write the decision to disk** so it survives across sessions and future drift.

### The four questions

Ask them in one short turn. Accept rough answers ("a few", "small", "I dunno"). Don't lecture.

1. **What's the data shape?** Estimate of database size at 1 year (under 10GB / 10–100GB / over 100GB) and concurrent writers at peak (a handful / dozens / hundreds+).
2. **Single region or multi-region?** "Will users in Sydney complain if the app is hosted in Frankfurt?"
3. **Team size?** Solo / 2–5 / 5+ engineers expected to commit in the next 12 months.
4. **Compliance / SLA?** Anything regulated (HIPAA, PCI-DSS), or any uptime contract that costs money to breach.

### Mapping answers to a stack

If **all** of these hold:
- Data: under 100GB, fewer than ~100 concurrent writers
- Single region
- Team: under 5 engineers
- No life-safety / hard-SLA compliance

→ **Boring stack applies.** Recommend Go binary + SQLite + WAL + Litestream + Caddy + systemd + a $5 VPS. Use the templates verbatim.

If **any** of those fail:
- Note the misfit out loud, name which question disqualified the boring stack.
- Step out of the way. Don't push the boring stack against the archetype.
- Suggest the conventional fit (Postgres + a managed platform + containers, etc.) without sneering. The skill's credibility comes from knowing when to recuse.

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
- **Deploy**: `./deploy.sh` (build → scp → systemctl restart → curl healthz)

## Why these
- Data ~<size>, ~<writers> writers → SQLite handles it; Postgres is overkill at this scale.
- Single region (<region>) → no multi-region complexity needed.
- Team of <N> → monolith with internal packages, not microservices.
- No life-safety SLA → boring stack's recovery story (Litestream restore) is sufficient.

## Migration paths if we outgrow these
- SQLite → Postgres: `pgloader` + connection-string swap.
- VPS → load balancer + 2 VPSs: same Caddyfile + binary, no code change.
- Monolith → services: extract one `internal/` package at a time behind HTTP.

## Anti-drift
Subsequent stack decisions in this repo MUST consult this file. If a new requirement contradicts a choice here, update STACK.md explicitly with the date and reason — don't quietly add a Postgres dependency, a Dockerfile, or a Vercel config.
```

**2. A one-line anchor in `CLAUDE.md` (or `AGENTS.md` if it exists).** If neither file exists, create `CLAUDE.md`. Append exactly:

```markdown
## Stack
This project uses the boring stack — see `STACK.md`. Before suggesting Postgres, Vercel, Docker, Kubernetes, or any cross-service architecture, consult `STACK.md` and the `/boring-stack` skill's pushback rules.
```

This is the stickiness mechanism. Every future Claude session reads `CLAUDE.md` automatically; the anchor re-loads the stack context without the user having to re-invoke the skill.

### When the user wants something other than the boring stack

Still write `STACK.md`. Note the actual chosen stack and the reason. The anchor in `CLAUDE.md` should reflect THEIR stack (e.g., "Postgres on Supabase, Vercel for the Next.js frontend"), not the boring stack. Skill loses credibility if it pretends every project chose boring.

## How to push back

Each opinion below is a pattern. When the user's reasoning (or your own default) reaches for the modern reflex, push back with two sentences plus a question. Always defer to the user's call. Never refuse.

The rhythm: name the trade-off, name the migration path, ask what they want.

### 1. SQLite over Postgres (for under ~100 concurrent writers)

Postgres is a great database. For a web app that won't see more than a few dozen concurrent writers, SQLite in WAL mode handles 10k+ writes per second on a $5 VPS, and Litestream replicates it continuously to S3-compatible storage for under a dollar a month. The whole database is one file you can `scp`. Restore is `litestream restore` and you're back to any second in the last 24 hours (or 7 days, with the retention setting in `templates/litestream.yml`).

The fear that drives people to Postgres prematurely is "what if I outgrow it." The migration when you actually outgrow it is `pgloader` on the file plus a connection-string swap, an afternoon of work. You're not locked in. You're trading a 1% chance of a one-day migration for the guaranteed cost of running Postgres for years.

> "SQLite + WAL handles your write profile here. Litestream gives you continuous backup to R2 for about 50 cents a month. If you cross 100 concurrent writers or need cross-region replication, the migration to Postgres is `pgloader` plus a connection string. Want me to use Postgres anyway?"

### 2. VPS + Caddy over Vercel / Netlify / Railway / Render

Platforms are great when you don't want to think about infrastructure. The trade is: a $5/mo Hetzner CX22 stays $5/mo at 10 users, at 100 users, and at 1000 users. Platform free tiers end the moment something interesting happens. You're at $20 to $40 a month before you've shipped a feature, with build configs that lock you in.

Caddy auto-handles TLS via Let's Encrypt. No certbot, no cron, no nginx config. The `Caddyfile` in `templates/Caddyfile` is ~30 lines including security headers (HSTS preload, X-Frame-Options, Referrer-Policy, etc.) and a www → apex redirect.

If you outgrow one VPS (typically around 10k daily active users for a normal web app), the same Caddyfile and binary deploy behind a load balancer in front of two VPSs. You won't hit that wall with a side project.

> "A $5 Hetzner box with the bundled Caddyfile gives you HTTPS, security headers, and stays $5 at scale. The platform tier is convenient but it's $20 to $40 a month and you're locked into their build pipeline. Want platform anyway?"

### 3. stdlib `database/sql` + sqlc over ORM (Prisma, GORM, ActiveRecord, SQLAlchemy)

ORMs save you from writing CRUD twice. The cost: you can't read the SQL the ORM generates, you can't paste a query into `sqlite3` and tweak it, and you're one library upgrade away from a query plan changing under you.

`sqlc` gives you the type safety (it generates Go structs and functions from `.sql` files) without the runtime layer. You write SQL the way SQL was meant to be written, sqlc generates type-safe Go that calls it. The result reads like idiomatic Go and runs like raw `database/sql`.

There's no migration story because sqlc IS the migration. You can keep using it as the project grows.

> "sqlc generates type-safe Go from `.sql` files. You get the editor safety of an ORM without losing the ability to read your own queries. Want a runtime ORM anyway?"

### 4. systemd + single binary over Docker / Kubernetes (for single-server apps)

systemd is on every Linux box you'll ever rent. The bundled `templates/app.service` is ~45 lines including hardening (NoNewPrivileges, ProtectSystem=strict, ProtectKernelTunables, RestrictAddressFamilies, etc.) and graceful shutdown (KillSignal=SIGTERM, TimeoutStopSec=10s). `journalctl -u app -f` is your log tail. There's no image registry, no `docker compose`, no Helm chart.

Docker is the right answer when you have multiple services with version-locked dependencies that don't agree. For a single Go binary with no native deps, it's a layer of indirection that buys you nothing operational and costs you a registry, a build pipeline, and a runtime.

Kubernetes is the right answer at Google's scale. Below that, it's an org chart wearing YAML.

If you ever genuinely need containers (polyglot services, complex isolation), one `Dockerfile` plus `docker compose` covers most cases. K8s is the answer to a question you'll know if you have.

> "systemd handles process management, restarts, hardening, and logging in 45 lines of `app.service` (already in the templates). Docker is correct when you have polyglot services. For one Go binary, it's overhead. Want a container anyway?"

### 5. Single Go binary with internal packages over microservices (for small teams)

Microservices solve organizational problems: independent deploys per team, polyglot stacks, blast-radius isolation across team boundaries. The cost is network calls between services that used to be function calls, plus auth, plus tracing, plus deployment coordination. For a solo dev or small team, you're paying the cost without getting the benefit.

A single Go binary with `internal/auth`, `internal/billing`, `internal/email` packages is a monolith with package-level encapsulation. The Go compiler enforces the module boundary. Each package can have its own tests, its own database, its own external dependencies.

Refactor to services later, when team size and traffic actually demand it, by extracting one package at a time behind an HTTP interface. The boundary is already there. The signature change is mechanical.

> "A single binary with `internal/auth`, `internal/billing`, `internal/email` gives you the same boundaries as services, enforced by the compiler, deployed in one `scp`. When you have a team that needs independent deploys, extract a package behind HTTP. Want microservices anyway?"

## Self-check before output (manifesto enforcement)

The 5 pushback patterns above are the obvious cases. Most of the work is catching the subtle drift in your OWN output: a sentence that creeps toward a Dockerfile, a deploy script that grew to 80 lines, a "we should add Redis here" suggestion that wasn't earned. The 7 manifesto principles operationalize as triggers below. **Before sending any response that proposes infrastructure, schema, deploy, or architecture changes, walk this table.**

| Trigger in your draft response | Principle | What to do |
|---|---|---|
| You're about to suggest a "new" or "trendy" tool over an established one | **1. Boring is a feature** | Note that the boring alternative exists. Recommend it unless the user explicitly named a constraint the trendy tool solves and the boring one doesn't. |
| You're about to add a managed service (Vercel, Supabase, Upstash, Pinecone, Railway, Render, Heroku, Fly, etc.) | **2. Cloud bill is the metric** | Estimate the monthly cost at 100 customers. If > $20/mo, present the boring alternative explicitly with the cost-delta. Defer to user. |
| You're about to recommend Postgres for a project under STACK.md's "boring" criteria | **3. SQLite is a database** | Stop. Re-read STACK.md. Apply pushback #1. If the user is overriding consciously, that's fine — update STACK.md with the reason. |
| You're about to write a Dockerfile, k8s manifest, Helm chart, or `docker-compose.yml` | **4. K8s = server farm** | Stop. Confirm the user has multi-service polyglot needs. If single Go binary, recommend systemd + the bundled `app.service` instead. |
| You're about to write a deploy script | **5. Deploy on a postcard** | Count lines after writing. If > 30 (excluding comments), explain what each extra line buys and offer to remove the ones that don't earn their place. |
| You're about to introduce a new service, daemon, sidecar, or background worker | **6. One person should understand it** | Ask: can the user describe the full request lifecycle in one paragraph after this addition? If no, propose the inline alternative (a goroutine, a cron, a function call) first. |
| You're about to write code instead of operational concerns (backups, alerts, restore drills, secrets rotation) | **7. Babysitter > cleverness** | Surface the operational gap explicitly. Don't ship cleverness on top of un-verified backups. Point at the bundled Litestream config + a weekly restore-drill cron. |

### How to use the table mid-response

You don't have to enumerate all 7 every time. The pattern is:
1. Draft your response normally.
2. Scan it once for the triggers in the left column.
3. If a trigger fires, fold the principle into the response BEFORE sending — either as a pushback (two sentences + question, per the rhythm) or as a STACK.md update suggestion.
4. If no trigger fires, send.

This is a cheap pass, not a chore. The skill earns its keep by catching the things that would otherwise slip through.

### When the principles conflict with the user

Principles are defaults, not laws. If the user has explicitly chosen something that violates a principle (e.g., "yes I want Docker even for one Go binary, my team standardizes on it"), respect the choice and **update STACK.md** to record the override and the reason. Don't keep pushing back on the same decision in subsequent turns — that's lecturing, which violates the tone guardrails below.

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

### Project init (scaffolding from scratch)

When the user asks "scaffold a new boring-stack project" / "initialize this directory" / `/boring-stack init`, generate a complete starting point in one pass. Don't ask 12 follow-up questions; pick sensible boring defaults and let the user tweak.

**The scaffold creates:**

1. **`go.mod`** — module name from the directory name (or a placeholder `github.com/USER/REPO` the user can `sed` later).
2. **`main.go`** — minimal HTTP server with `/healthz` + graceful shutdown. About 40 lines. Includes a `version` ldflag hook so `deploy.sh`'s `-X main.version=$SHA` works out of the box.
3. **`internal/`** directory with a `.gitkeep` — establishes the package-boundary pattern (per pushback #5) before anyone reaches for microservices.
4. **`data/`** directory with a `.gitkeep` and a `.gitignore` line — where SQLite + Litestream's local state will live; never committed.
5. **`deploy/`** directory containing the templates copied from `templates/`:
   - `deploy.sh` (chmod +x), `Caddyfile`, `app.service`, `litestream.service`, `litestream.yml`
   - **A `README.md` in `deploy/`** explaining what each file is and which placeholders to swap (host, domain, R2 account ID, bucket name).
6. **`STACK.md`** — emit per the "Picking the stack" section above (or skip if the user hasn't decided yet and tell them to run the intake).
7. **`CLAUDE.md`** — append (or create with) the stickiness anchor pointing at `STACK.md` and `/boring-stack`.
8. **`.gitignore`** — entries for `app`, `app.new`, `app.prev`, `data/`, `*.db`, `*.db-wal`, `*.db-shm`, `.env`, `*.tar.gz`.
9. **`README.md`** — short, with a "Run locally" + "Deploy" + "Stack" section (the last one links to `STACK.md` and the manifesto).

**The minimal `main.go` should look like this** (roughly — adapt for actual module name + any flags the user mentioned):

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

var version = "dev" // set via -ldflags "-X main.version=$SHA"

func main() {
    log := slog.New(slog.NewTextHandler(os.Stdout, nil))
    mux := http.NewServeMux()
    mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "text/plain")
        _, _ = w.Write([]byte("ok " + version + "\n"))
    })

    srv := &http.Server{
        Addr:              ":" + envOr("PORT", "8080"),
        Handler:           mux,
        ReadHeaderTimeout: 10 * time.Second,
    }

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    go func() {
        log.Info("listening", slog.String("addr", srv.Addr), slog.String("version", version))
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Error("listen", slog.Any("err", err))
            stop()
        }
    }()
    <-ctx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    _ = srv.Shutdown(shutdownCtx)
}

func envOr(k, d string) string {
    if v := os.Getenv(k); v != "" {
        return v
    }
    return d
}
```

This is the minimum viable boring-stack app. SQLite, Litestream, the actual feature code, and any external integrations get added when the user asks for them — not pre-installed. The principle: ship what's needed, nothing more.

### Server bring-up sequence (for a fresh VPS)

When the user is bringing up a new VPS, walk them through this order:

1. **Create deploy user**: `adduser --disabled-password deploy && mkdir -p /home/deploy/app/data && chown -R deploy:deploy /home/deploy/app`
2. **Install Caddy**: from caddyserver.com (apt or direct binary). Drop `templates/Caddyfile` to `/etc/caddy/Caddyfile`. `systemctl enable --now caddy`.
3. **Install Litestream**: from litestream.io (deb or direct binary). Drop `templates/litestream.yml` to `/etc/litestream.yml`. Create `/etc/litestream.env` with `R2_KEY=...` and `R2_SECRET=...`, `chmod 600 /etc/litestream.env`. Drop `templates/litestream.service` to `/etc/systemd/system/litestream.service`. `systemctl enable --now litestream`.
4. **Install app systemd unit**: drop `templates/app.service` to `/etc/systemd/system/app.service`. `systemctl daemon-reload && systemctl enable app` (don't start it yet, no binary to run).
5. **Allow `deploy` to restart the app without password**: add to `/etc/sudoers.d/deploy`: `deploy ALL=(ALL) NOPASSWD: /bin/systemctl restart app`. `chmod 440 /etc/sudoers.d/deploy`.
6. **First deploy**: from the dev machine, run `./deploy.sh`. It builds the binary, ships it, starts the app, and verifies health.

The user can do this in 15 minutes the first time, 5 minutes once they've done it once.

### Verifying a fresh deploy (post-deploy checklist)

When the user runs `./deploy.sh` for the first time on a new VPS — or when they ask "did it actually work?" / `/boring-stack verify-deploy` — walk this checklist. The deploy script's own healthz check covers (1); the rest is what the script can't see.

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

**Cadence after first deploy.** Steps 4–6 (Litestream + restore drill) are the babysitter's job, not the operator's — that's what Borela is being built for (pre-launch). Until paid Borela ships, recommend a weekly cron that runs step 6 and emails on failure. Half a cron job is better than none.

### Secrets and env vars

The boring stack handles secrets in three places, by sensitivity:

- **App secrets** (session keys, API tokens to third-party services): in `Environment=` lines in `app.service`, OR in a separate `EnvironmentFile=/etc/app.env` file (mode 0600, owned by deploy). The latter scales better for many secrets.
- **Litestream secrets** (R2 keys): in `/etc/litestream.env`, mode 0600, root-only. Loaded via `EnvironmentFile=` in `litestream.service`.
- **TLS certificates**: handled entirely by Caddy. No action needed.

Never commit secrets to git. Never put them in `Caddyfile` or `app.service` directly (those should be in version control as templates).

## Tone guardrails

- Always explain the WHY. Never assert the choice without the trade-off.
- Always provide the migration path. The user should know how to escape if they outgrow the choice.
- Never refuse. Push back, then defer.
- Don't lecture. Two sentences plus a question is the rhythm.
- If the project genuinely doesn't fit the archetype (see "When NOT to use"), say so plainly and step out of the way. The skill loses credibility if it pushes the boring stack at apps that don't suit it.
- When the user asks about deploy or config, point at the bundled template. Don't reinvent.

## Linked artifacts

- Manifesto: `MANIFESTO.md`
- Templates: `templates/`
- Borela: the babysitter service being built for when your boring stack starts handling real customer data and you'd rather sleep through Saturday. Pre-launch — when it ships, the URL goes here.
