# Contributing

Thanks for caring enough to send a patch. The boring stack stays boring on purpose, so contributions should respect a few constraints:

## What's in scope

- **Bug fixes** in the templates (`templates/`) — wrong path, missing flag, security issue, broken on a recent Caddy/Litestream/systemd version.
- **New AI tool support** in `docs/add.sh` — if your favorite tool isn't covered, add a detect + install branch following the pattern of the existing ones. Keep the marker-gated append idempotent.
- **Sharper pushback rules** in `SKILL.md` — if Claude (or another agent) keeps drifting toward a modern reflex you've caught it doing, propose the trigger and the response, in the same "two sentences plus a question" rhythm as the existing five.
- **Manifesto refinements** if a principle is genuinely unclear — but the count stays at seven. Adding an eighth requires a strong case.

## What's out of scope

- **More tools in the templates.** The five files cover ~95% of single-server Linux web apps. Adding a sixth is one too many — and "but it's nice for X" is exactly how scope creep happens.
- **Switching templates to a different stack** (e.g., "what about Postgres + Docker?"). That's a fork, not a contribution. The whole point of this skill is its opinion.
- **Adding emojis to user-facing text.** Just don't.

## How to send a change

1. Fork the repo, branch off `main`.
2. Make the change. Keep the diff tight — one concern per PR.
3. If you touched `SKILL.md`, smoke-test by re-running `docs/install.sh` locally and confirming the file ends up where you expect.
4. If you touched `docs/add.sh`, run it in a clean temp directory with `--tool all` and confirm every file lands without error.
5. Open the PR. Title in the imperative ("add Cursor support to add.sh"). Body explains the why.

## Releases

`CHANGELOG.md` is the source of truth for what shipped when. New tag = new entry under `## [x.y.z] — YYYY-MM-DD`.

## License

MIT — same as the rest of the repo. By submitting a patch you agree your contribution is licensed under the same terms.
