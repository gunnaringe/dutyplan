---
name: verify
description: Verify changes (the Go app and the Python planner) as cheaply as possible with the mise tasks. Use after changing code or tests, and before a commit.
---

Run only through mise (never python/uv/pytest directly). All tasks are quiet: one line on success.

1. While you work: `mise run test:fast` (about 5 s: the Go tests + the Python rule tests). Go only: `mise run test:app -- -run Name`.
2. Before you are done: `mise run test` (about 15 s, everything).
3. Changed or added a **rule**: `mise run mutations -- --only <word from the name>` (seconds), and finally
   `mise run mutations` (about 7 min). Quiet on success; prints only SURVIVED / OUTDATED / INVALID.
4. Before a commit: `mise run verify` (tests + all mutations + smoke test of the built app).
5. Changed the frontend (`web/`): `mise run app:ui` (headless browser, about 20 s, English and Norwegian; it also fails if a page becomes wider than a phone screen). Skip it for changes that are not visible.

If a mutation survived, a test that depends on the rule is missing. Write it so that independent terms do not cover for it:
switch off other terms with parameters (`NO_FAIR` in tests/helpers.py, `w_combo: {impractical: 0}`), and test both directions
when the outcome could otherwise come out right by a tie.

A new planner rule = four places: `model.py`, the independent `check.py`, a test, and a line in `PY_MUTATIONS` in
`scripts/mutations.py`. A new Go rule (especially organisation scoping): a test + a line in `GO_MUTATIONS`; every lookup must be scoped by the organisation. Update `PLAN.md` only in the section that applies.
A new text for people: a key in both languages (`web/i18n.js`, `internal/i18n`); a new error code also needs `err.<code>` in `web/i18n.js`. A test checks all of it.

Do not read `fixtures/autumn2026.json` or `uv.lock` (large, generated). Use `mise run plan -- --info`.
