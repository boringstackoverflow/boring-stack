# Templates

Production-hardened reference configs the `/boring-stack` skill ships with. Each file already includes the security/operability work that's easy to forget — drop it in, swap the placeholders, ship.

## Backend / deploy

| File | Purpose |
|---|---|
| `deploy.sh` | Build, ship, swap, verify, rollback. ~10 lines. |
| `Caddyfile` | TLS-terminating reverse proxy with security headers. ~30 lines. |
| `app.service` | Hardened systemd unit (NoNewPrivileges, ProtectSystem=strict, etc.). |
| `litestream.service` | systemd unit for the Litestream replication daemon. |
| `litestream.yml` | Litestream config: SQLite DB path + R2 destination. |

## Frontend (starter)

| File | Purpose |
|---|---|
| `index.html.tmpl` | Minimal Go `html/template` page. Receives `ProjectName` and `Version`, loads `static/app.css`, and loads htmx from a pinned unpkg URL. `Version` cache-busts the stylesheet. |
| `static/app.css` | Starter stylesheet (~50 lines, modern CSS, no framework). Edit freely — it's a starting point, not a UI kit. |

The frontend starter follows the boring-stack default: server-rendered HTML + htmx (the Go-shaped equivalent of Rails Hotwire). One `<script>` tag, no `node_modules`, no build step. See SKILL.md §6 ("Server-rendered HTML + htmx over SPA frameworks") for the full trade-off and the three-tier staging (Stage 1 here; Stage 2 adds an embedded Preact+HTM widget for one rich page; Stage 3 adds a chosen SPA toolchain for genuinely SPA-shaped products while keeping the backend boring).

To self-host htmx instead of loading from CDN: download `https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js` to `static/htmx.min.js` and follow the comment in `index.html.tmpl`.
