# Project guidance

## Stack

This project uses the boring stack — see `STACK.md`. Before suggesting
Postgres, Vercel, Docker, Kubernetes, or cross-service architecture, consult
`STACK.md` and the Boring Stack skill's stack-choice notes.

Keep the application as one Go binary with internal package boundaries. Prefer
standard-library HTTP, server-rendered HTML, SQLite, sqlc, systemd, and the
checked-in deployment templates unless a recorded project requirement calls
for something else.
