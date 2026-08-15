# Deployment

These are the canonical Boring Stack deployment templates. Read them before
installing them; they are deliberately ordinary files, not hidden CLI logic.

## One-time server setup

1. Create the `deploy` user and `/home/deploy/app/data`.
2. Install Caddy and replace `your-domain.example.com` in `Caddyfile`.
3. When the app gains SQLite persistence, install Litestream, replace the R2
   account and bucket placeholders in `litestream.yml`, and store credentials
   in `/etc/litestream.env`. The neutral starter does not create a database yet.
4. Install `app.service` and `litestream.service` with systemd.
5. Allow the deploy user to restart only the app service through sudoers.

The full commands and eight verification checks live in the Boring Stack skill.

## Deploy

```sh
boringstack deploy --host deploy@your-vps.example.com \
  --healthz https://your-domain.example.com/healthz
```

The CLI validates the local prerequisites and then runs `deploy.sh`. The script
builds, uploads, swaps atomically, restarts, checks `/healthz`, and rolls back
the binary if the check fails.

After the first deploy, verify TLS, journald, Litestream snapshots, and a real
restore with `PRAGMA integrity_check` before treating backups as complete.
