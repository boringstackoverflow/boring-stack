---
title: "What broke in launch week"
date: 2026-05-19
summary: "From launch day until 48 hours ago, every newsletter signup on boringstack.org silently failed at the TLS edge. Lower bound: 49 attempts. Ceiling: unknowable. Here's the bug, the blast radius, the 3 PRs we shipped to fix it, and the 3 things we'd do differently."
draft: false
---

From May 7 to May 17, every newsletter signup on boringstack.org silently failed.

Caddy's port-80 access log shows at least **49 attempts** that got as far as the HTTPS redirect before dying. That's the floor. The true number is **higher and unknowable** — most modern browsers POST directly over HTTPS, where the TLS handshake collapsed before any HTTP request was ever logged. None of those emails reached the database. They're not recoverable.

This is the postmortem.

## What broke

`/etc/caddy/Caddyfile` on the VPS had a server block for `borela.dev` and **no server block for `api.boringstack.org`**. Caddy received the HTTPS connections, had nothing to serve them with, and failed the TLS handshake. The browser showed a TLS error or a "can't reach this site" message. The user closed the tab. No 4xx, no 5xx, no log line, nothing.

The newsletter signup form on boringstack.org (a static GitHub Pages site) POSTs to `https://api.boringstack.org/v1/newsletter/subscribe`. So the form had been silently broken since the moment the static site went live — the entire launch week, the HN comments, the Reddit traffic, the X thread — everything pointed at an endpoint that couldn't complete a TLS handshake.

The Caddyfile block was supposed to be added by `scripts/setup.sh`:

```bash
# from setup.sh:
#   - APPENDS api.boringstack.org block to /etc/caddy/Caddyfile
#     (only if not already present — borela.dev block left untouched)
```

But the version of `setup.sh` that ran on the VPS during initial provisioning didn't have that section yet. The current `setup.sh` did. Nobody re-ran it. Boring stack, boring oversight.

## Why we didn't notice

Three honest reasons, in order of how much each one cost us:

1. **No synthetic check on the signup endpoint.** A 60-second cron that POSTs a test email to `/v1/newsletter/subscribe` and exits non-zero on anything but HTTP 202 would have paged us within a minute of the form going live broken. We didn't have it.
2. **`n=1` confirmed subscriber.** That was me. I subscribed manually via the CLI to test the send loop on launch day. A flat zero on the public form was indistinguishable from "the launch hasn't quite caught fire yet." If we'd had three honest signups expected from internal testing, the gap would have been obvious within hours.
3. **The first newsletter send was also broken.** A separate bug (the worker's `MaxAge` guard interpreting frontmatter `date: 2026-05-07` as midnight UTC, then refusing to send anything older than 24h by the time GH Pages built and the worker polled) meant *I* didn't receive the launch issue either. So "no one's getting the newsletter" looked like "nothing's been sent" rather than "the signups never happened."

Three independent failure modes, lined up like dominoes. Any one of them surfacing in isolation would have caught the others. None did.

## What we shipped

Three PRs, ordered by what unblocks what:

**1. The Caddyfile block.** The immediate fix. Appended the missing `api.boringstack.org` block to `/etc/caddy/Caddyfile`, `caddy validate`-d it (so a bad edit couldn't take down the live `borela.dev` block too), then `systemctl reload caddy`. First HTTPS request triggered ACME, Let's Encrypt issued a cert in ~10 seconds, the form started accepting POSTs.

**2. `scripts/setup.sh` rewritten as truly idempotent.** So the underlying cause — "setup.sh changed after it had already run on the box, and nobody re-ran it" — can't bite the same way twice. Marker-delimited managed Caddyfile block (`# >>> bsb-setup ... # <<<`) so re-running the script *updates* the block in place instead of doing nothing. Write-only-if-changed for the systemd unit and the sudoers fragment. `caddy validate` before reload, with backup-and-restore on failure. The script now does the right thing whether it's the first run or the fiftieth.

**3. Build tool emits `pubDate` from build time, not frontmatter midnight.** This is the *other* bug that helped this one hide. The site generator was setting `<pubDate>` to `<frontmatter date>T00:00:00Z`, which meant any issue pushed more than 24h after that midnight got eaten by the worker's `MaxAge` guard — so the launch issue itself never went out, and "no signups + no newsletter delivery" looked like one problem instead of two. Now the generator reads the existing `feed.xml` and *preserves* each issue's pubDate by slug; new issues get the current build time. Historical pubDates are byte-identical across rebuilds (no feed churn), and the `MaxAge` cliff is closed at the root.

Layered on top: bumped `NEWSLETTER_MAX_AGE` from 24h to 72h on the VPS as a guardrail. With (3) shipped, that guardrail should never trigger again — but defense in depth costs nothing.

## What we'd do differently

1. **The synthetic check first, the launch second.** A 30-line cron-curl-and-log script against `/v1/newsletter/subscribe` would have caught this in 60 seconds. Ship it before the form goes public, not after.
2. **Run `setup.sh` again on every meaningful infra change.** The marker-delimited managed-block pattern only helps if you re-run the script. Add it to the deploy checklist or, better, fold it into `make deploy`.
3. **Track "expected baseline traffic" as a metric.** A graph of `signups/day` that bottoms out at zero for a week should look as alarming as a graph of `errors/day` going through the roof. Right now we have the latter and not the former.

## The ask

If you tried to subscribe to this newsletter any time between May 7 and May 17 and never heard back — **that was us, not you.** The form works now. [Try again](/#newsletter).

If you didn't try, also subscribe. It's still one practical field note every Tuesday.

## The deal, again

This is issue #4. Next Tuesday: **"SQLite is not a toy database"** — the case for SQLite at small-app scale, pushed forward one week so this postmortem could land while it's still fresh.

Boring is not infallible. Boring is *recoverable*. Receipts include the bad ones.

[Star the repo](https://github.com/boringstackoverflow/boring-stack). Reply with your stack. I read every reply.
