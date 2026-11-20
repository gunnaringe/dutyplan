# dutyplan

Shares out duties among events with hard requirements, preferences and an even distribution. A general tool (not only for churches).
Private GitHub repo: `gunnaringe/dutyplan` (the folder and the Go module `github.com/gunnaringe/dutyplan` have the same name; the Norwegian page calls the
program *Tjenesteplan*, and the binary, the Python package and `DUTYPLAN_SOLVER` keep the `dutyplan` name). Keep it private, and check that nothing personal has been added before you push (see
«Testing and data»). `README.md` is the user-facing documentation (what it is, install, flags, backups, the no-login warning); keep it true when behaviour changes.

Status: the planner (`solver/`, Python) is connected to the app (Go + SQLite + vanilla JS): organisations, plans, people, tasks, combinations, groups, events with
their own places, details rows, answers, drafts (runs) with set-by-hand places, locks and selections, a pinned current plan, rename and delete, a page in English and
Norwegian. Since 2026-11-19 the page follows the sketches: **one table per plan** (details, tasks with places, the draft allocation, availability), dialogs for the rest,
and an organisation page with the plans and groups (the setup page is gone). There is no change log and no name/actor (removed 2026-11-19), and no day types.
Since 2026-11-20 the API covers only what the page uses: no personal links, no «My assignments», no bulk task paste, no cells or selections for several
events at once, and no places or description when an event is created (adding repeatedly with `until` and `every`, copying and the availability map stay: the page uses them).
A first test runs on kate since 2026-11-20 (asked by the user): the image from `Dockerfile` (`mise run app:image`), behind Pocket ID at
dutyplan.haxxor.xyz, manifests and the import steps in `clusters/kate/dutyplan/`. No registry, no backup yet.

Layout: `cmd/dutyplan` (main), `internal/store` (SQLite, the schema, all data access), `internal/planner` (builds the input from the database,
starts the Python planner, turns the answer into a view with explanations), `internal/api` (JSON API + serves the frontend), `internal/i18n` (server texts),
`web/` (index.html, i18n.js, app.js, style.css, embedded in the binary), `solver/` (Python/CP-SAT), `scripts/` (smoke test, browser check).

## Run cheaply (read this first)

- **Everything goes through mise**: `mise install` (python, uv, go), `mise run setup` (Python packages), then `mise tasks`. Never python/uv/pytest/go directly.
  Skills: `verify` (tests + mutations), `run-planner` (the planner) and `run-app` (the app). The tasks are quiet and print about 1–8 lines.
- **Start the app for a person: `mise run up`** (setup + build + run; seeds the demo organisation into a new database). For your own checks use `mise run app:run -- --addr ... --db /tmp/x.db`.
- The usual loop: change → `mise run test:fast` (Go + Python rules, about 5 s) → `mise run test` (everything, about 15 s) → after a rule change
  `mise run mutations -- --only <word>` (the whole set takes about 7 min) → before a commit `mise run verify` (tests + all mutations + smoke test of the app).
  For a change to the page also `mise run app:ui` (browser check in both languages, about 20 s).
- **The Python mutations edit `solver/dutyplan_solver/` in place** while they run (restored at the end, on Ctrl-C and on SIGTERM; a leftover is restored from
  `solver/.mutation-backup/` by the next run). Never run tests, the smoke test or the app against the real planner at the same time; `verify` runs the mutations last for this reason.
- **Do not read** `PLAN.md` in full (about 300 lines). `grep -n '^## ' PLAN.md`, read only the section you need.
  Do not read `solver/fixtures/autumn2026.json` or `solver/uv.lock` (large, generated); use `mise run plan -- --info`.
- Do not run `--full`, `measure:*` or `app:ui` without a reason; they are expensive or need sibling dependencies.
- Never put test output through `| tail` inside an `&&` chain: the pipe hides a failing exit code.
- The app finds the planner through `DUTYPLAN_SOLVER` (set in `mise.toml`, needs `mise run setup`); without it the two tests against the real
  solver are skipped and «Plan» is switched off. The mutations run the Go tests without it, so rules must be caught by the tests with a fake solver.
- File paths for `plan`, `solve` and `check` are relative to the repo root. New Python packages: `mise run add -- package` (and `mise run lock`);
  `UV_LOCKED=1` otherwise blocks changes to `uv.lock`.

## Design rules from the user (do not break without asking)

- **End to end first** (2026-11-20): make what exists work end to end before adding features. No individual user management (confirmed 2026-11-20).
- **No users yet, so no migrations** (2026-11-20): change `internal/store/migrations/001_schema.sql` in place; an old database is refused at startup
  and is simply deleted. No upgrade code and no backward compatibility until the user says the app is live.

- **Organisation → plans.** Everything belongs to an organisation; an organisation has several plans (for example «Autumn 2026»). The organisation is in the address
  (`/o/<org>/p/<plan>`, `/api/orgs/{org}/...`), the server has no «current organisation». Every lookup is scoped by the organisation, and
  a plan/person from another organisation gives 404 (the isolation tests and the `tenant:` mutations guard this). No login.
- **All writes go through `store.write`** (one transaction). Do not write to the database any other way. There is no change log and no actor: do not add them back without asking.
- **Renaming changes the name only.** The slug is the address (made from the name when the organisation or plan is created; the API takes no slug); links must keep working. Never make a rename change a slug.
- **The API covers only what the page uses** (2026-11-20: "Update api to only cover what the UI uses. We will rather start simple and expand."). Do not add an endpoint or a
  field the page does not call; add it together with the page change that needs it. **No individual user management**: no personal links, no per-person pages or endpoints,
  no accounts. A person is a row in the organisation (name, task choices, can be proposed) and in each plan's table.
- **Deleting a plan** is refused while a run is in progress. An organisation is deleted only when it has no plans, and the caller confirms with its slug (the API handler
  writes the deletion to the server log). `TestEveryTableWithOrgOrPlanDataIsKnownToTheDeletions` fails when a table is added that the deletions do not know about: update `DeletePlan`/`DeleteOrg` and the test.
- **The event is the unit, not the date.** Events are self-contained: two can fall on the same day. An event can have a start time (and an end time) and a description; the planner does not use them, so they must not change the input or hash of a run. Availability («can I»), combinations, places
  and group busyness apply to the event. The planner's «dates» are events (the id is arbitrary text).
- **Pin = the light form of publishing.** One finished run per plan can be pinned as the current plan; the page opens with it (else the newest run).
  Pinned runs are never pruned. There is still no approval flow or publishing.
- **One table per plan, one view** (2026-11-19, from the sketches): events across (date, time, ⧉ copy; **+** adds one), then four sections:
  *Event details* (the plan's rows, `plan_field`/`event_field`: ⋮⋮ drags a row, ✎ renames/moves/deletes it; text for people, never sent to the planner, not in the hash),
  *Tasks* (the places per event, typed into the cells; empty = not needed), the green *Draft allocation* (the run on show merged with what is set by hand), and
  *Availability* (a count per event and one row per person). Clicking a date, task, person, row ✎ or draft place opens its dialog; ⚙ *Plan settings* has the name, the
  weighting, the earlier drafts (Show/Pin) and Delete. Do not add separate pages for these again.
  - **On a phone** (the sketch's option A, chosen 2026-11-20): the same page with *Availability* turned and moved above the rest: events down, people across
    (first names; the event column stays put), the person's tasks in the draft under each answer. The rest of the table follows, sideways-scrolling. One layout is
    in the page at a time (`PHONE` in `web/app.js`, the same 40rem as the phone rule in `style.css`); there is no «my» column and no list of one's own.
  - **No day types.** Each event has its own places (`event_task`); in the planner's input every event is its own type. Events are copied instead (`POST .../events/{id}/copy`:
    dates, or repeated every week or every 2–8 weeks until a date (`every`; only the dates are made, nothing stores the rhythm), with times, details, places and
    names set by hand as chosen; answers and drafts are never copied).
  - **The draft:** a place set by hand (a manual cell that is not locked) has a violet background; a planner name that was locked is a cell with `locked` and shows a
    «locked» chip; «maybe» marks a place given against a ?; an unfilled place says «Not filled · Why?» and the place dialog explains.
  - Answers: **blank = not answered** (neutral for the planner); a click cycles blank → good → poor → never → blank.
- **No spreadsheet support** (neither import nor export). The user does not want it; do not suggest it again.
- **No calendar feed** (`.ics`, subscription or download; dropped 2026-11-09). Do not suggest it again, although events have times of day now (since 2026-11-09) and a run can be pinned: the user has not asked for it again.
- **A general tool.** Tasks, people, groups, events and details rows are data the admin creates. No church concepts in code, data model or rules (examples in placeholders and docs are generic).
- **No special rules for single things.** A **group** is a named set of people. It can be in a manual cell or proposed by the planner
  (`group_tasks`); the members are then busy, and the shared combination setting (task × task) decides what else they can have the same day.
  A **selection** (only these people/groups can be picked) covers «their own sound engineer» and the like. No skills or conditional requirements.
- **Can be proposed** is a setting per person and per group (default yes). No = the planner never picks them; manual cells are fine.
- **One click per setting** (2026-11-20, the user's choice): a person's task cycles *not → bad → good* (`poor`/`good`, not = no row, the start), two tasks
  on the same event *can't → bad → good* (`cannot`/`practical`, bad = no row = impractical). *Neutral* tasks, *preferred* pairs and the plan setting
  `default_combination` are gone. There is **no «active»** any more (removed 2026-11-20): «Can be proposed» is the only switch per person; people who
  are not taking part are placed in a section (planned).
- **Group work = one assignment per member** in the distribution (overall and per task).
- **Optimise only within one plan** (2026-11-09). No history from earlier plans: the app never sends `history` or `previous_plan` to the planner.
- **Removed at the user's request:** a maximum of tasks per person per day, and rest between events. Do not put them back without asking.
- **No login in the first version.** No links with a secret, no roles, and no change log (removed 2026-11-19).
- Each person chooses which tasks they take part in; one list per person with answers per event (*does not suit / poor / good*), an optional own list per task (not built).
- «Even distribution» = **equal total load** (the user's answer 2026-11-09): someone who only takes Video should not end up with half of someone who takes Sound and Video.
  Distribution per task is off by default (`w_task_fair`/`w_task_range` 0). Do not make the per-task term the main term again without asking.
- **The weights are configuration per plan** (2026-11-09): `store.WeightSpecs` (default and limits), stored as what differs from the default, and part of the hash. The defaults must be copies of
  the planner's (`--defaults`; a test checks it). A new weight = one line in `WeightSpecs`, a label and a hint (`weight.<key>.label`/`.hint`) in both languages in `web/i18n.js`, and a group in `WEIGHT_GROUP` in `web/app.js`.

## Languages and texts

- The repo, its docs, code, comments and tests are in **English**. The page is in English and Norwegian (bokmål); the browser's language is the default and the header has a switch.
- **On the wire everything is English and ASCII**: an answer per event `never|poor|good` (blank = not answered), a person's task `poor|good` (no row = not),
  a combination `cannot|practical` (no row = `impractical`, «bad»), a group's task `poor|neutral|good`; error kinds and codes. Texts for people are made at the edge.
- **The page** (`web/i18n.js`): `MESSAGES = {en: {...}, nb: {...}}`, one `'key': 'text',` per line, `{placeholders}`, `.one`/`.other` for anything that counts (`tn`). Use `’` (not `'`) inside texts.
  `web/i18n_test.go` checks that both languages have the same keys and placeholders, that every key `app.js` names exists, and that every error code the server can send has `err.<code>` (and that the text only uses parameters the server sends).
- **The server** (`internal/i18n`): explanations, warnings, compromises and notes are stored as codes with arguments and written at view time in the language of `?lang=`. A new text = a key in both `en.go` and `nb.go`
  (a test checks parity).
- **Errors** are `store.Invalid(code, params, englishMessage)` / `store.Conflict(...)`; the body is `{error, code, params, message}`. A new code needs `err.<code>` in both catalogues of `web/i18n.js`.
- Page addresses are English: `/o/<org>` and `/o/<org>/p/<plan>`; anything else is «Page not found».

## Testing and data

- `check.py` is an independent checker; keep it independent of `model.py` (only the default values are shared).
- Every rule must have a test that fails when the rule is broken in the model: `solver/scripts/mutations.py` (`PY_MUTATIONS`, `GO_MUTATIONS`) checks it. An outdated pattern (`OUTDATED`) is fixed, never deleted, and the number of mutations must not drop.
  **The one exception** (the user's choice, 2026-11-20): a mutation may be deleted only when **the user asked for the feature it guards to be removed**, and the
  commit message names the mutations and that request. A pattern that is merely hard to fix, or a rule you think is no longer needed, is not a reason.
  History: 2026-11-19: 248 → 256 with the day types and the change log gone; 2026-11-20: 256 → 238 with the per-person API, places set when an event is
  created and cells for several events at once gone; 2026-11-20: 238 → 242 with repeating every few weeks.
- No real names, e-mail addresses or phone numbers in the repo, not even the church's, the programmes' or the spreadsheet's names, and no known names from the spreadsheet (scrubbed 2026-11-09, and the history was rewritten). Invented names must not be real first names from the original crew: check against the spreadsheet before using new names. Dates and event types in the test data are invented (2026-11-09). All that remains of the shape of the spreadsheet is the task names (common roles such as Sound and Video) and roughly how large the teams are. The test data has invented names and is made by `mise run fixture`.
- The test data is `solver/fixtures/autumn2026.json`, made by `solver/fixtures/make_autumn2026.py` (`mise run fixture`).
- **The pictures in `README.md`** are `docs/img/*.png`, made by `mise run app:shots` (about 20 s, `scripts/shots.js`) from the real app on invented data (the community hall «Maple Hall»; the people are the invented names of `scripts/seed-demo.sh`, so the rule above about names holds). Run it again, and commit the pictures, when the page changes visibly; it removes the old ones first. The day in the pictures is fixed (21 October 2015: the script rewrites the stamps in its temporary database), so a run without a visible change gives byte-identical files. `web/docs_test.go` checks that every picture the README shows exists and that none is left unused (it skips in the mutation copies).
- Do not give a script a name that collides with the standard library (`inspect.py` broke the numpy import).
- Do not run `mise lock` here: `lockfile` in this configuration wrote a `mise.lock` in `~/code` (the parent configuration). The versions are pinned in `mise.toml`.
