---
title: "A private dashboard for SQLite pool waits"
date: 2026-11-24
summary: "Embed a private page in your Go binary to see whether work queues for a SQLite connection during a slowdown."
draft: false
---

<!-- newsletter-plan: 29; scheduled: 2026-11-24 -->

Suppose a CSV import is running and saves in your Go app suddenly take longer. Add a private `/ops` page that reads `db.Stats()` from the connection pool your saves already use. Refresh it before and during the import. A rising wait count tells you Go queued at least one request for a connection, and you can read that counter while every connection is occupied because the page runs no SQL. [Go pool statistics](https://pkg.go.dev/database/sql#DBStats) · [Go pool implementation](https://go.dev/src/database/sql/sql.go#L1220)

## Put the page on a second listener

The Boring Stack scaffold embeds `templates/*.html.tmpl` and parses those files into `pageTemplates`. Its reference Caddyfile sends every path on the app hostname to `127.0.0.1:8080`, so a dashboard registered on the main router would sit behind the public proxy. Give `/ops` its own router on `127.0.0.1:9091` and reach it over SSH. [Go embed documentation](https://pkg.go.dev/embed) · [Caddy request matchers](https://caddyserver.com/docs/caddyfile/matchers)

This example assumes you’ve already added SQLite through `database/sql` to a scaffolded app. Save the Go file as `ops.go` beside `main.go`. In `main`, after creating your long-lived `db` and `log`, add `ops := startOps(db, log)` followed by `defer ops.Close()`. Pass the write handle if you keep separate read and write pools.

The route uses Go 1.22 pattern syntax, which the scaffold already uses. The current stable download as of November 24, 2026, is Go 1.27.4. [Go routing reference](https://pkg.go.dev/net/http#ServeMux) · [Go downloads](https://go.dev/dl/)

```go
package main

import (
	"bytes"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

func startOps(db *sql.DB, log *slog.Logger) *http.Server {
	started := time.Now().UTC().Format(time.RFC3339Nano)
	mux := http.NewServeMux()
	mux.HandleFunc("GET 127.0.0.1/ops", func(w http.ResponseWriter, _ *http.Request) {
		pool := db.Stats()
		data := struct {
			Version  string
			Started  string
			Sampled  string
			WaitTime string
			Pool     sql.DBStats
		}{
			Version:  version,
			Started:  started,
			Sampled:  time.Now().UTC().Format(time.RFC3339),
			WaitTime: pool.WaitDuration.String(),
			Pool:     pool,
		}

		var page bytes.Buffer
		if err := pageTemplates.ExecuteTemplate(&page, "ops.html.tmpl", data); err != nil {
			log.Error("render ops page", "error", err)
			http.Error(w, "render ops page", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(page.Bytes())
	})

	srv := &http.Server{
		Addr:              "127.0.0.1:9091",
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen for ops requests", "addr", srv.Addr, "error", err)
		}
	}()
	return srv
}
```

Save the next file as `templates/ops.html.tmpl`, then rebuild. Start the binary locally and load `/ops` before deploying. The scaffold calls `template.Must` at startup, so a syntax error in the template can keep the whole app from starting. If port `9091` is taken on the VPS, the app logs the bind failure and can still serve public requests. [Go template reference](https://pkg.go.dev/html/template#Must)

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>SQLite connection pool</title>
</head>
<body>
  <h1>SQLite connection pool</h1>
  <p>Release {{.Version}}. Dashboard started {{.Started}}.</p>
  <p>Sampled {{.Sampled}}.</p>
  <dl>
    <dt>Connections in use</dt><dd>{{.Pool.InUse}}</dd>
    <dt>Open connection limit</dt>
    <dd>{{if .Pool.MaxOpenConnections}}{{.Pool.MaxOpenConnections}}{{else}}Unlimited{{end}}</dd>
    <dt>Connection waits</dt><dd>{{.Pool.WaitCount}}</dd>
    <dt>Total time from ended waits</dt><dd>{{.WaitTime}}</dd>
  </dl>
</body>
</html>
```

After deployment, run this from your laptop with an SSH account allowed to forward ports. `-L` with the explicit bind address puts the laptop end of the tunnel on loopback, and `-N` skips running a remote command. `ExitOnForwardFailure=yes` catches a failure to bind the local port. The HTTP request through the tunnel checks whether Go is actually listening at the destination. [OpenSSH client manual](https://man.openbsd.org/ssh#L) · [OpenSSH forwarding option](https://man.openbsd.org/ssh_config#ExitOnForwardFailure)

```sh
ssh -N -o ExitOnForwardFailure=yes \
  -L 127.0.0.1:9091:127.0.0.1:9091 deploy@your-vps.example.com
```

Open `http://127.0.0.1:9091/ops`. Use the numeric address because the route matches that HTTP host. Local processes on either machine can reach their respective loopback listener, so use machines you control and keep the page to aggregate counts and release metadata. Request `/ops` on the public HTTPS hostname too. With the reference Caddyfile and scaffold router, it should return `404`. [Go routing reference](https://pkg.go.dev/net/http#ServeMux)

## Read the change between refreshes

`InUse` shows how much pool capacity is occupied when you sample. Compare it with the open connection limit, where zero means Go has no configured cap. `WaitCount` rises when a checkout queues, even when the caller eventually gives up. `WaitDuration` adds elapsed time when a wait ends, including on cancellation, so the count can rise while the duration stays flat. Compare samples from the same run and pool. [Go connection limit](https://pkg.go.dev/database/sql#DB.SetMaxOpenConns) · [Go checkout implementation](https://go.dev/src/database/sql/sql.go#L1361)

In a pool capped at one, an import transaction can hold the only connection while a save waits before its SQL runs. The counters cover every caller on that handle, so a jump during the import gives you a place to investigate. Check what happens between `BeginTx` and `Commit`. Moving CSV parsing before `BeginTx` can return the connection sooner while keeping the writes in one transaction. [Go transactions](https://go.dev/doc/database/execute-transactions)

If the count stays flat from before a slow save until it returns, look for time spent after checkout. A save can already have a connection and still contend for SQLite’s write lock. WAL allows one writer at a time. [SQLite WAL concurrency](https://sqlite.org/wal.html#concurrency)

The page needs no collector, and every sample depends on someone opening it. An afternoon wait total gives you no way to place a spike at lunchtime. A restart resets the counters, and a stopped app takes the page with it.

When you need a timeline, register `collectors.NewDBStatsCollector(db, "sqlite")` for the same handle and serve it at a private `/metrics` endpoint. Version 1.25.0 of the Go client requires Go 1.26 or later. Let Prometheus scrape over a private connection from another host so samples it already collected survive an app restart or VPS outage. [Go collector reference](https://pkg.go.dev/github.com/prometheus/client_golang@v1.25.0/prometheus/collectors#NewDBStatsCollector) · [Client release notes](https://github.com/prometheus/client_golang/releases/tag/v1.25.0) · [Prometheus overview](https://prometheus.io/docs/introduction/overview/)

On a test database, record the wait count before your next import and refresh the page while you try a save.
