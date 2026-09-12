---
title: "The two health checks a one-VPS app actually needs"
date: 2026-09-12
summary: "Your process is running. Your users still cannot reach it. Two small probes distinguish a healthy Go process from a working public HTTPS path."
draft: false
---

Your service says `active (running)`. You curl localhost and get `ok`. A reader opens your site and gets a TLS error.

All three can be true at once.

In [launch week](2026-05-19-what-broke-in-launch-week.html), Boring Stack's public API had no matching Caddy server block. Newsletter signups failed at the TLS edge, before an HTTP request could reach the application. A check against localhost would have missed the broken layer entirely.

A running process is one fact. A working path from a user's machine is another. Check both.

## Two probes, two jobs

| Probe | Run it from | What it tells you |
|---|---|---|
| Local application check | The app's VPS, directly against Go | The process can answer a request. A separate readiness check can verify a required database read. |
| Public HTTPS check | Another machine | This location can reach the expected application through DNS, the network, TLS, and Caddy. |

These are starting points. Neither proves that a signup completes or that a payment succeeds. They do give you a useful first split when someone says the site is down.

## Start with the endpoint you already have

The [current scaffold](https://github.com/boringstackoverflow/boring-stack/blob/main/scaffold/base/main.go.tmpl) includes this handler:

```go
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    _, _ = w.Write([]byte("ok " + version + "\n"))
})
```

Check it on the VPS:

```sh
curl -fsS --connect-timeout 1 --max-time 3 \
  http://127.0.0.1:8080/healthz
```

A development build returns `ok dev`. A release build can return its injected version. Use the port your app actually listens on.

This checks whether Go can serve the request. The scaffold has no database yet, so this endpoint cannot honestly claim database health.

Once useful requests depend on SQLite, add a separate readiness endpoint that performs one bounded read against an actual application table. Choose a query with a known result, such as reading the expected migration version. Give it a deadline and return a failure status if it cannot finish or the schema is wrong. Opening a connection alone does not prove your application data is usable.

Keep the process check shallow. If an optional email provider is down, restarting an otherwise healthy Go process will not repair it. A database readiness failure should alert you or block a deploy; automatic restarts need their own justification.

## Check the public path from somewhere else

Run this from a machine other than the app's VPS:

```sh
curl -fsS --connect-timeout 2 --max-time 5 \
  https://app.example.com/healthz
```

Use the real public hostname. Keep certificate verification enabled. A probe that skips TLS verification cannot catch the certificate problem your browser is reporting.

HTTP success alone is still a weak contract. A misrouted proxy can serve a perfectly valid page from the wrong application. Check the status and expected body together:

```sh
#!/bin/sh
set -eu

url=${1:-https://app.example.com/healthz}
expected=${EXPECTED_HEALTH:-ok dev}
body=$(mktemp)
trap 'rm -f "$body"' EXIT
trap 'exit 1' HUP INT TERM

status=$(curl -fsS --connect-timeout 2 --max-time 5 \
  -o "$body" -w '%{http_code}' "$url") || exit 1

[ "$status" = 200 ] && [ "$(cat "$body")" = "$expected" ]
```

Save it as `check-app.sh` and invoke it with `sh`. Set `EXPECTED_HEALTH` to the deployed response. If you match a release identifier, update the monitor's expectation with each deploy. Otherwise, define a stable application-specific health response and verify the version separately in your deployment check.

The script rejects redirects, error statuses, and unexpected bodies. Its [curl timeouts](https://curl.se/docs/manpage.html#--max-time) bound the connection and overall request; those values are chosen defaults, not a measured availability guarantee.

## A failed command still needs a human

A script on your laptop checks the site only while your laptop runs it. A timer on the primary VPS stops when that VPS disappears.

Schedule the public probe on an independent machine or external monitor. Start with an interval appropriate to how quickly you need to know. Route failure to a notification channel you already read, include the hostname and time, and send a recovery notification when the check passes again.

A nonzero exit in an unattended journal is not an alert. Verify the notification path using a disposable failing URL before relying on it. Also watch for missing probe runs: an external monitor can fail too.

## The failure mode: green on localhost

When the local check passes and the public check fails, investigate the path between the probe and the process: DNS, firewall, certificate, proxy routing, and the probe's own network. Rebuilding the binary is not the first move.

When both fail, inspect the service and host. When both pass but signups fail, the probes have reached their limit: add a controlled check of the signup flow. A healthy `/healthz` never promised that every route works.

## When to outgrow this

One outside location gives you one outside perspective. When the product needs faster detection, several regions, or shared on-call ownership, move scheduling and alerting to monitoring that supports those requirements. Keep the useful endpoint contracts. Add redundancy when the availability requirement warrants it.

The first improvement is small enough to make today: check the app directly, check it through the public hostname from elsewhere, and make sure a failure reaches you.

**Which layer can fail while your current health check still says green?**
