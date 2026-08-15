# Expense tracker scaffold eval

This is the repeatable acceptance scenario for the agent-first Boring Stack
journey. Run it from an empty temporary directory with the current skill and
CLI installed.

## Prompt

> Build me a small expense-tracking SaaS using Boring Stack. Keep it single-user
> for this first slice. I need to create, list, edit, and delete expenses with a
> category, date, description, and amount; show this month's total; include
> useful empty and validation states; and add a seed command. Run it locally and
> show me the result. Do not add authentication, teams, billing, or uploads yet.

If the agent asks the five-question intake, answer:

- Under 1 GB after one year and a handful of concurrent writers.
- One region; users are primarily in North America.
- Solo project.
- No regulated data or contractual uptime SLA.
- Forms and pages.

## Required behavior

The result must:

1. Start from `boringstack init` or `boringstack new` rather than a second
   handwritten scaffold.
2. Stay one Go binary with standard-library HTTP and server-rendered HTML.
3. Store expenses in SQLite with an append-only migration and sqlc-generated
   queries.
4. Store money as integer minor units, never floating point.
5. Implement create, list, edit, and delete with server-side validation.
6. Show the current month's total and useful empty/error states.
7. Include a deterministic seed subcommand.
8. Include handler and store tests against a temporary real SQLite database.
9. Pass `go test ./...` and show a working local page.
10. Preserve the generated stack record and deployment artifacts.

## Failure conditions

- Adds Postgres, an ORM, Docker, a SPA framework, or another service without a
  requirement that earns it.
- Only changes the generated welcome-page copy.
- Claims “SaaS” completeness while silently omitting the requested expense
  operations or validation.
- Invents auth, Stripe, teams, uploads, analytics, or email.
- Does not run the result.

## Timing and scoring

Target: working local product in under ten minutes in a warm agent environment.

- 2 points: correct scaffold and stack boundaries.
- 4 points: complete expense behavior and money/date correctness.
- 2 points: real SQLite/sqlc tests and clean build.
- 1 point: visible local result.
- 1 point: no unrequested product or infrastructure scope.

A release candidate should score at least 9/10 on three consecutive clean runs.
