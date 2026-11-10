---
title: "Useful Go logs with the standard library"
date: 2026-11-10
summary: "Give each important operation one completion event with a precise outcome, stable fields, and deliberately limited data."
draft: false
---

<!-- newsletter-plan: 27; scheduled: 2026-11-10 -->

A log line saying `save failed` cannot tell you which operation failed or which release produced it. For a small Go app, start with one completion event per important application operation: a stable name, an outcome, and a few chosen fields. Emit it where you know the final result.

The current [Boring Stack scaffold](https://github.com/boringstackoverflow/boring-stack/blob/main/scaffold/base/main.go.tmpl) already constructs `slog.New(slog.NewTextHandler(os.Stdout, nil))` and records listening, listen failures, and shutdown failures. Its lifecycle logging is a starting point. Application events still need meanings you can rely on.

## Define success before writing the log call

For an example expense app, define `expense_save` as the result of one save attempt after validation. An `ok` outcome means the [transaction committed successfully](https://go.dev/doc/database/execute-transactions). A successful insert followed by a failed commit does not qualify; neither does starting background work that has yet to save anything.

If you retry a busy database internally, emit the completion event after retries finish. Otherwise, one successful save can look like several failed operations. Give the caller responsible for the final result ownership of this event, rather than logging the same returned error in every function it passes through.

Keep the message constant and put changing values in attributes. Start with service, release version, a server-generated request ID, outcome, and elapsed milliseconds. A numeric `duration_ms` stays comparable across events; mixing seconds, strings, and milliseconds under one key makes later queries unreliable.

Go's [slog introduction](https://go.dev/blog/slog) documents both built-in handlers: text keeps attributes readable at a terminal, while JSON preserves numeric types for downstream tools. The APIs used here arrived in Go 1.21; the [current stable download](https://go.dev/dl/) is Go 1.27.3.

## Record the result without copying the request

Example: save this helper as `internal/applog/log.go` and create the logger once with `New(version)`. Capture `started := time.Now()` before the save operation, then call `SaveResult` once with its final error, after any commit attempt and retries. The caller must still handle that error and return the appropriate response.

```go
package applog

import (
	"context"
	"log/slog"
	"os"
	"time"
)

func New(version string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil)).
		With("service", "app", "version", version)
}

func SaveResult(ctx context.Context, log *slog.Logger,
	requestID string, started time.Time, err error,
) {
	level, outcome := slog.LevelInfo, "ok"
	if err != nil {
		level, outcome = slog.LevelError, "failed"
	}

	log.LogAttrs(ctx, level, "expense_save",
		slog.String("request_id", requestID),
		slog.String("outcome", outcome),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
}
```

Replace `app` with your service name. When replacing the scaffold's logger, remove the listening event's explicit version attribute, because `With` already supplies it. Pass this logger to the code handling saves.

The default minimum level is Info, so both outcomes pass. `LogAttrs` accepts typed attributes, and passing a context does not automatically extract a request ID: this helper adds it explicitly. These behaviors are documented in the [slog API reference](https://pkg.go.dev/log/slog).

The missing error string is deliberate. It reduces accidental disclosure at the cost of diagnostic detail; a failed event alone cannot distinguish a lock timeout from a full disk. When that distinction matters, add a fixed `error_kind` classification based on documented error types or codes, with an `unknown` fallback. Avoid classifying errors by matching their message text.

Choose other fields by what you will investigate. A route template identifies a handler without recording a URL containing customer identifiers. Keep request bodies, cookies, authorization headers, magic links, and SQL parameter values out of these events. [OWASP's logging guidance](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html) identifies tokens, passwords, and sensitive personal data as information to exclude or protect.

The easy failure is adding `"error", err` or an entire request struct for convenience. Error strings can contain submitted values, and JSON encoding does not redact them. Review failure-path values before adding a field.

## Know what the event cannot prove

The [service template](https://github.com/boringstackoverflow/boring-stack/blob/main/templates/app.service) sends stdout and stderr to the journal. With this setup, JSON `level` remains inside the message; journal priority is a separate value. Do not assume journal priority filters select your JSON Error events: [systemd defaults stdout messages to Info](https://raw.githubusercontent.com/systemd/systemd/main/man/systemd.exec.xml).

There is also a gap between committing and logging: the process can die between those calls. The built-in handlers write synchronously, and [Logger discards handler errors](https://pkg.go.dev/log/slog#Handler). A returned logging call therefore proves neither durable storage nor a complete history. If the product requires a record of each change, write an audit record in the same database transaction.

When you need off-host retention or searches across machines, forward the journal to a collector that parses the JSON. Keep the event names and field meanings. If profiling shows logging delaying requests, replace the handler or collection path and decide explicitly whether a full buffer blocks requests or drops events.

Pick one database write today and define exactly what its success event promises before adding the log call.
