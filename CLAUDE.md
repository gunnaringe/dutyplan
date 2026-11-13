# dutyplan

Shares out duties among events with hard requirements, preferences and an even distribution. A general tool (not only for churches).
Private GitHub repo: `gunnaringe/dutyplan` (the folder and the Go module `github.com/gunnaringe/dutyplan` have the same name; the Norwegian page calls the
program *Tjenesteplan*, and the binary, the Python package and `DUTYPLAN_SOLVER` keep the `dutyplan` name). Keep it private, and check that nothing personal has been added before you push (see
«Testing and data»). `README.md` is the user-facing documentation (what it is, install, flags, backups, the no-login warning); keep it true when behaviour changes.

Status: the planner (`solver/`, Python) is connected to the app (Go + SQLite + vanilla JS): organisations, plans, people, change log, setup (tasks, day types with places,
combinations, events, groups), «My page», «Plan» on the plan, manual cells, selections and «Lock» (phase 4), grid and «My assignments» (phase 5), run history with a pinned current plan,
rename and delete of organisations and plans, rename of people and tasks, and a page in English and Norwegian. Deployment on kate (phase 6) is postponed at the user's request (2026-11-09): do not
start on it unless asked.

Layout: `cmd/dutyplan` (main), `internal/store` (SQLite, migrations, all data access), `internal/planner` (builds the input from the database,
starts the Python planner, turns the answer into a view with explanations), `internal/api` (JSON API + serves the frontend), `internal/i18n` (server texts),
`web/` (index.html, i18n.js, app.js, style.css, embedded in the binary), `solver/` (Python/CP-SAT), `scripts/` (smoke test, browser check).

## Run cheaply (read this first)

- **Everything goes through mise**: `mise install` (python, uv, go), `mise run setup` (Python packages), then `mise tasks`. Never python/uv/pytest/go directly.
  Skills: `verify` (tests + mutations), `run-planner` (the planner) and `run-app` (the app). The tasks are quiet and print about 1–8 lines.
- **Start the app for a person: `mise run up`** (setup + build + run; seeds the demo organisation into a new database). For your own checks use `mise run app:run -- --addr ... --db /tmp/x.db`.
- The usual loop: change → `mise run test:fast` (Go + Python rules, about 5 s) → `mise run test` (everything, about 15 s) → after a rule change
  `mise run mutations -- --only <word>` (the whole set takes about 7 min) → before a commit `mise run verify` (tests + all mutations + smoke test of the app).
  For a change to the page also `mise run app:ui` (browser check in both languages, about 20 s).
- **Do not read** `PLAN.md` in full (about 300 lines). `grep -n '^## ' PLAN.md`, read only the section you need.
  Do not read `solver/fixtures/autumn2026.json` or `solver/uv.lock` (large, generated); use `mise run plan -- --info`.
- Do not run `--full`, `measure:*` or `app:ui` without a reason; they are expensive or need sibling dependencies.
- Never put test output through `| tail` inside an `&&` chain: the pipe hides a failing exit code.
- The app finds the planner through `DUTYPLAN_SOLVER` (set in `mise.toml`, needs `mise run setup`); without it the two tests against the real
  solver are skipped and «Plan» is switched off. The mutations run the Go tests without it, so rules must be caught by the tests with a fake solver.
- File paths for `plan`, `solve` and `check` are relative to the repo root. New Python packages: `mise run add -- package` (and `mise run lock`);
  `UV_LOCKED=1` otherwise blocks changes to `uv.lock`.

## Design rules from the user (do not break without asking)

- **Organisation → plans.** Everything belongs to an organisation; an organisation has several plans (for example «Autumn 2026»). The organisation is in the address
  (`/o/<org>/p/<plan>`, `/api/orgs/{org}/...`), the server has no «current organisation». Every lookup is scoped by the organisation, and
  a plan/person from another organisation gives 404 (the isolation tests and the `tenant:` mutations guard this). No login.
- **All writes go through `store.write`** (needs an actor in `X-Actor`, logs in the same transaction). Do not write to the database any other way.
  **The one exception is `DeleteOrg`**: an organisation's change log goes with it, so the deletion is written to the server log (slog) by the API handler instead.
- **Renaming changes the name only.** The slug is the address; links (also the personal ones people were sent) must keep working. Never make a rename change a slug.
- **Deleting a plan keeps the change log** (the lines lose their plan link and get the plan's name in front) and is refused while a run is in progress. An organisation is deleted only
  when it has no plans, and the caller confirms with its slug. `TestEveryTableWithOrgOrPlanDataIsKnownToTheDeletions` fails when a table is added that the deletions do not know about: update `DeletePlan`/`DeleteOrg` and the test.
- **The event is the unit, not the date.** Events are self-contained: two can fall on the same day. An event can have a start time (and an end time) and a description; the planner does not use them, so they must not change the input or hash of a run. Availability («can I»), combinations, places
  and group busyness apply to the event. The planner's «dates» are events (the id is arbitrary text).
- **Pin = the light form of publishing.** One finished run per plan can be pinned as the current plan; «My assignments» follows the pinned run, else the newest finished one. Pinned runs are never pruned.
  There is still no approval flow or publishing.
- **No spreadsheet support** (neither import nor export). The user does not want it; do not suggest it again.
- **No calendar feed** (`.ics`, subscription or download; dropped 2026-11-09). Do not suggest it again, although events have times of day now (since 2026-11-09) and a run can be pinned: the user has not asked for it again.
- **A general tool.** Tasks, people, groups, events and day types are data the admin creates. No church concepts in code, data model or rules (examples in placeholders and docs are generic).
- **No special rules for single things.** A **group** is a named set of people. It can be in a manual cell or proposed by the planner
  (`group_tasks`); the members are then busy, and the shared combination setting (task × task) decides what else they can have the same day.
  A **selection** (only these people/groups can be picked) covers «their own sound engineer» and the like. No skills or conditional requirements.
- **Can be proposed** is a setting per person and per group (default yes). No = the planner never picks them; manual cells are fine.
- **Group work = one assignment per member** in the distribution (overall and per task).
- **Optimise only within one plan** (2026-11-09). No history from earlier plans: the app never sends `history` or `previous_plan` to the planner.
- **Removed at the user's request:** a maximum of tasks per person per day, and rest between events. Do not put them back without asking.
- **No login in the first version.** No links with a secret, no roles. A change log instead.
- Each person chooses which tasks they take part in; one list per person with answers per event (*does not suit / poor / good*), an optional own list per task (not built).
- «Even distribution» = **equal total load** (the user's answer 2026-11-09): someone who only takes Video should not end up with half of someone who takes Sound and Video.
  Distribution per task is off by default (`w_task_fair`/`w_task_range` 0). Do not make the per-task term the main term again without asking.
- **The weights are configuration per plan** (2026-11-09): `store.WeightSpecs` (default and limits), stored as what differs from the default, and part of the hash. The defaults must be copies of
  the planner's (`--defaults`; a test checks it). A new weight = one line in `WeightSpecs`, a label and a hint (`weight.<key>.label`/`.hint`) in both languages in `web/i18n.js`, and a group in `WEIGHT_GROUP` in `web/app.js`.

## Languages and texts

- The repo, its docs, code, comments and tests are in **English**. The page is in English and Norwegian (bokmål); the browser's language is the default and the header has a switch.
- **On the wire everything is English and ASCII**: levels `never|poor|neutral|good`, combinations `cannot|impractical|practical|preferred`, error kinds and codes. Texts for people are made at the edge.
- **The page** (`web/i18n.js`): `MESSAGES = {en: {...}, nb: {...}}`, one `'key': 'text',` per line, `{placeholders}`, `.one`/`.other` for anything that counts (`tn`). Use `’` (not `'`) inside texts.
  `web/i18n_test.go` checks that both languages have the same keys and placeholders, that every key `app.js` names exists, and that every error code the server can send has `err.<code>` (and that the text only uses parameters the server sends).
- **The server** (`internal/i18n`): explanations, warnings, compromises and notes are stored as codes with arguments and written at view time in the language of `?lang=`. A new text = a key in both `en.go` and `nb.go`
  (a test checks parity).
- **Errors** are `store.Invalid(code, params, englishMessage)` / `store.Conflict(...)`; the body is `{error, code, params, message}`. A new code needs `err.<code>` in both catalogues of `web/i18n.js`.
- Page addresses are English: `/o/<org>`, `/o/<org>/setup`, `/o/<org>/p/<plan>`, `/o/<org>/p/<plan>/me/<id>`.
- Old change-log lines written before 2026-11-09 may be Norwegian; the log shows what was stored.

## Testing and data

- `check.py` is an independent checker; keep it independent of `model.py` (only the default values are shared).
- Every rule must have a test that fails when the rule is broken in the model: `solver/scripts/mutations.py` (`PY_MUTATIONS`, `GO_MUTATIONS`) checks it. An outdated pattern (`OUTDATED`) is fixed, never deleted; the number of mutations must not drop.
- No real names, e-mail addresses or phone numbers in the repo, not even the church's, the programmes' or the spreadsheet's names, and no known names from the spreadsheet (scrubbed 2026-11-09, and the history was rewritten). Invented names must not be real first names from the original crew: check against the spreadsheet before using new names. Dates and event types in the test data are invented (2026-11-09). All that remains of the shape of the spreadsheet is the task names (common roles such as Sound and Video) and roughly how large the teams are. The test data has invented names and is made by `mise run fixture`.
- The test data is `solver/fixtures/autumn2026.json`, made by `solver/fixtures/make_autumn2026.py` (`mise run fixture`).
- **The pictures in `README.md`** are `docs/img/*.png`, made by `mise run app:shots` (about 30 s, `scripts/shots.js`) from the real app on invented data (the community hall «Maple Hall»; the people are the invented names of `scripts/seed-demo.sh`, so the rule above about names holds). Run it again, and commit the pictures, when the page changes visibly; it removes the old ones first. `web/docs_test.go` checks that every picture the README shows exists and that none is left unused (it skips in the mutation copies).
- Do not give a script a name that collides with the standard library (`inspect.py` broke the numpy import).
- Do not run `mise lock` here: `lockfile` in this configuration wrote a `mise.lock` in `~/code` (the parent configuration). The versions are pinned in `mise.toml`.
