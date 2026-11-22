# dutyplan

Shares out duties among events with hard requirements, preferences and an even distribution. A general tool (not only for churches).
Public GitHub repo (since 2026-11-20, the user's choice): `gunnaringe/dutyplan` (the folder and the Go module `github.com/gunnaringe/dutyplan` have the same name; the Norwegian page calls the
program *Tjenesteplan*, and the binary, the Python package and `DUTYPLAN_SOLVER` keep the `dutyplan` name). Everything pushed is public: check that nothing personal has been added before you push (see
«Testing and data»). `README.md` is the user-facing documentation (what it is, install, flags, backups, signing in); keep it true when behaviour changes.

Status: the planner (`solver/`, Python) is connected to the app (Go + SQLite + vanilla JS): organisations, plans, people, tasks, combinations, sections, events with
their own places, details rows, answers, drafts (runs) with set-by-hand places, locks and selections, a pinned current plan, rename and delete, a page in English and
Norwegian. Since 2026-11-19 the page follows the sketches: **one table per plan** (details, tasks with places, the draft allocation, availability), dialogs for the rest,
and an organisation page with the plans (the setup page is gone). There is no change log and no name/actor (removed 2026-11-19), and no day types.
Since 2026-11-20 the API covers only what the page uses: no personal links, no «My assignments», no bulk task paste, no cells or selections for several
events at once, and no places or description when an event is created (adding repeatedly with `until` and `every`, copying and the availability map stay: the page uses them).
**CI** (GitHub Actions, since 2026-11-20): `.github/workflows/ci.yml` runs `mise run lint`, `test`, `app:smoke`, `app:ui` (playwright-core from npm, the runner's
Chrome) and, in a job of its own, `mise run mutations`, on every PR and on main. **`main` is protected** (ruleset «main», 2026-11-20): changes only through a PR, the checks `test`, `mutations` and `build` must pass before
it can merge, no force push or deletion; no review is required. Secret scanning with push protection is on. `.github/workflows/image.yml` builds the image
(amd64 + arm64; the Go stage cross-compiles) on PRs and publishes `ghcr.io/gunnaringe/dutyplan` from main (`main`, `main-<UTC>-<sha>`, `sha-<sha>`) and `v*` tags.
A first test runs on kate since 2026-11-20 (asked by the user), behind Pocket ID at dutyplan.haxxor.xyz, manifests in `clusters/kate/dutyplan/`:
kate runs the published image through Flux image automation (`clusters/kate/flux-system/dutyplan-image.yaml`), with a nightly SQLite backup to NFS.

Layout: `cmd/dutyplan` (main), `internal/store` (SQLite, the schema, all data access), `internal/planner` (builds the input from the database,
starts the Python planner, turns the answer into a view with explanations), `internal/api` (JSON API + serves the frontend), `internal/i18n` (server texts),
`web/` (index.html, i18n.js, app.js, style.css, embedded in the binary), `solver/` (Python/CP-SAT), `scripts/` (smoke test, browser check).

## Run cheaply (read this first)

- **Everything goes through mise**: `mise install` (python, uv, go, node, prek), `prek install` (the hooks of `prek.toml`), `mise run setup` (Python packages), then `mise tasks`. Never python/uv/pytest/go directly.
  Skills: `verify` (tests + mutations), `run-planner` (the planner) and `run-app` (the app). The tasks are quiet and print about 1–8 lines.
- **Start the app for a person: `mise run up`** (setup + build + run; seeds the demo organisation into a new database). For your own checks use `mise run app:run -- --addr ... --db /tmp/x.db`.
- The usual loop: change → `mise run test:fast` (Go + Python rules, about 5 s) → `mise run test` (everything, about 15 s) → after a rule change
  `mise run mutations -- --only <word>` (the whole set takes about 7 min) → before a commit `mise run verify` (`lint` = prek: gofmt, JS syntax, YAML/JSON/TOML, keys, big files; tests; all mutations; smoke test of the app).
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

- **End to end first** (2026-11-20): make what exists work end to end before adding features.
- **Migrations from 002 on** (2026-11-22): signing in came as `002_access.sql`, because kate's database already had 001 and the move to
  Contabo with public signup was close. Until the user says the app is live an old database may still be deleted; but do not change an
  applied migration, add the next number.

- **Organisation → plans.** Everything belongs to an organisation; an organisation has several plans (for example «Autumn 2026»). The organisation is in the address
  (`/o/<org>/p/<plan>`, `/api/orgs/{org}/...`), the server has no «current organisation». Every lookup is scoped by the organisation, and
  a plan/person from another organisation gives 404 (the isolation tests and the `tenant:` mutations guard this).
- **Signing in** (2026-11-22, the user: "Make it use proper oidc ... let people sign up with new organization themselves"): `internal/api/auth.go`,
  OpenID Connect (code + PKCE, go-oidc) against one provider; in production the Pocket ID at id.sermon-studio.app, shared with sermon-studio
  ("User management and login will be the same"). Session = random token in a host-only `__Host-dutyplan` cookie, its SHA-256 in `session`.
  `access` (middleware) is the one place that decides: pages, `/api/health` and `/api/me` open; the rest needs a session (401 `signed_out`); an
  organisation's routes need a member (`membership`: admin or member), 404 for anyone else. **Roles** (2026-11-22, the user: "the roles are org
  admin and org members + edit and view shared link"; migration 004): an admin renames and deletes the organisation, changes members and makes or
  deletes invitations (403 `admin_only` for others); a member does everything else, plans and links to plans included. Whoever
  creates an organisation is its admin; a taken address gets `-2`, `-3` (never a 409 that tells the name exists). `--no-login` (or listening on
  loopback without `--oidc-issuer`) = no login, everyone may do everything: the tests, the scripts and kate (behind its forward-auth). The
  `access:` mutations guard it.
- **Sharing** (2026-11-22, the user: "possible to create shared links that allow access to the plan or org"): `share_link` (003), `internal/api/access.go`.
  A view or edit link is to **one plan** (no links to a whole organisation since 004); `join` is an invitation (opened signed in = member). Opening `/s/<token>` keeps the
  token in the `__Host-dutyplan-links` cookie (up to 20), signed in or not; `access` lets a request through on a link when no membership does. A
  plan link reaches its plan and the organisation's shared parts (people, tasks, sections; the plan page needs them), never the plan list,
  other plans, members or links, and its stream hears nothing of other plans (`grant.sees`). A link never renames or deletes the organisation.
  Members make and delete plan links; a link never makes links (it does not reach `/links`); only an admin invites and changes members; anyone may
  leave; the last admin stays. Tokens are stored as they are, so a link can be copied again. `GET /orgs/{org}` says `role` / `level` / `plan`, which the page uses to show or hide. The `share:` mutations guard it.
- **Limits** (2026-11-22, for the public signup): caps in `internal/store/limits.go` (`atMost`, code `too_many_<what>` with `max`; checked
  in the transaction that creates), and a rate limit on changes per person or address in `internal/api/ratelimit.go` (only with a login;
  `--client-ip-header` names the proxy's header; `costly()` lists the dearer creations). Only `/` with a login may be indexed. The
  `limits:` and `index:` mutations guard them.
- **The header** (2026-11-22, the user, "align the design to be similar to sermon studio"): brand, organisation, then round icon links
  to the organisation's **Members** page (`/o/<org>/members`, members with a login) and **Settings** page (`/o/<org>/settings`: rename and
  delete, admins), the language button (`EN`/`NB`, a menu) and the person signed in as initials (a menu with their name and a Sign out
  button). Buttons in the header are one height (`app:ui` checks it). Members and settings are pages, not dialogs.
- **All writes go through `store.write`** (one transaction). Do not write to the database any other way. There is no change log and no actor: do not add them back without asking.
- **Renaming changes the name only.** The slug is the address (made from the name when the organisation or plan is created; the API takes no slug); links must keep working. Never make a rename change a slug.
- **The API covers only what the page uses** (2026-11-20: "Update api to only cover what the UI uses. We will rather start simple and expand."). Do not add an endpoint or a
  field the page does not call; add it together with the page change that needs it. **A person in a plan is a row, not an account**: no personal links, no per-person pages or endpoints for
  the people who are planned (name, task choices, can be proposed). Accounts are only for those who plan (see *Signing in*); links that share a
  whole organisation or plan are built (see *Sharing*).
- **Deleting a plan** is refused while a run is in progress. An organisation is deleted only when it has no plans, and the caller confirms with its slug (the API handler
  writes the deletion to the server log). `TestEveryTableWithOrgOrPlanDataIsKnownToTheDeletions` fails when a table is added that the deletions do not know about: update `DeletePlan`/`DeleteOrg` and the test.
- **The event is the unit, not the date.** Events are self-contained: two can fall on the same day. An event can have a start time (and an end time) and a description; the planner does not use them, so they must not change the input or hash of a run. Availability («can I»), combinations, places
  apply to the event. The planner's «dates» are events (the id is arbitrary text).
- **Pin = the light form of publishing.** One finished run per plan can be pinned as the current plan; the page opens with it (else the newest run).
  Pinned runs are never pruned. There is still no approval flow or publishing.
- **One table per plan, one view** (2026-11-19, from the sketches): events across (date, time, ⧉ copy; **+** adds one), then four sections:
  *Event details* (the plan's rows, `plan_field`/`event_field`: ⋮⋮ drags a row, ✎ renames/moves/deletes it; text for people, never sent to the planner, not in the hash),
  *Tasks* (the places per event, typed into the cells; empty = not needed; ⋮⋮ drags a task, in the organisation's order `task.position`, not in the planner's input;
  *Fill row* in the task's dialog sets every event of the plan), the green *Draft allocation* (the run on show merged with what is set by hand), and
  *Availability* (a count per event and one row per person; ⋮⋮ drags a person, `person.position`; organisation-wide *sections* (`person_section`)
  are headings with their people below, those without a section first; only placement, never in the planner's input; on a phone the order stays, without headings). Clicking a date, task, person, row ✎ or draft place opens its dialog; ⚙ *Plan settings* has the name, the
  weighting, the earlier drafts (Show/Pin) and Delete. Do not add separate pages for these again.
  - **On a phone** (the sketch's option A, chosen 2026-11-20): the same page with *Availability* turned and moved above the rest: events down, people across
    (first names; the event column stays put), the person's tasks in the draft under each answer. The rest of the table follows, sideways-scrolling. One layout is
    in the page at a time (`PHONE` in `web/app.js`, the same 40rem as the phone rule in `style.css`); there is no «my» column and no list of one's own.
  - **No day types.** Each event has its own places (`event_task`); in the planner's input every event is its own type. Events are copied instead (`POST .../events/{id}/copy`:
    dates, or repeated every week or every 2–8 weeks until a date (`every`; only the dates are made, nothing stores the rhythm), with times, details, places and
    names set by hand as chosen; answers and drafts are never copied).
  - **The draft:** a place set by hand (a manual cell that is not locked) has a violet background and a ✎ square (`mark('hand')`); a planner name that was locked is
    a cell with `locked` and shows a lock square (`mark('lock')`, 2026-11-22, the user: "a square with lock"); the legend uses the same squares; «maybe» marks a place given against a ?; an unfilled place says «Not filled · Why?» and the place dialog explains.
  - Answers: **blank = not answered** (neutral for the planner); a click cycles blank → good → poor → never → blank (thumb up, ?, thumb down).
- **No spreadsheet support** (neither import nor export). The user does not want it; do not suggest it again.
- **No calendar feed** (`.ics`, subscription or download; dropped 2026-11-09). Do not suggest it again, although events have times of day now (since 2026-11-09) and a run can be pinned: the user has not asked for it again.
- **A general tool.** Tasks, people, events and details rows are data the admin creates. No church concepts in code, data model or rules (examples in placeholders and docs are generic).
- **No groups** (removed 2026-11-20 at the user's request; building them back is issue #9). A place set by hand is a person or a text.
- **No special rules for single things.** A **selection** (only these people can be picked) covers «their own sound engineer» and the like. No skills or conditional requirements.
- **Can be proposed** is a setting per person (default yes). No = the planner never picks them; manual cells are fine. On the page it is the ⚡ button by
  the name in the table (2026-11-20), not in the person dialog.
- **One click per setting** (2026-11-20, the user's choice): a person's task cycles *can't → if needed → good* (red thumb down → yellow ? → green thumb up; `poor`/`good`, can't = no row, the start), two tasks
  on the same event *can't → bad → good* (`cannot`/`practical`, bad = no row = impractical). *Neutral* tasks, *preferred* pairs and the plan setting
  `default_combination` are gone. There is **no «active»** any more (removed 2026-11-20): «Can be proposed» is the only switch per person; people who
  are not taking part are placed in a section.
- **Optimise only within one plan** (2026-11-09). No history from earlier plans: the app never sends `history` or `previous_plan` to the planner.
- **Removed at the user's request:** a maximum of tasks per person per day, and rest between events. Do not put them back without asking.
- **No change log** (removed 2026-11-19). Roles: admin and member only.
- Each person chooses which tasks they take part in; one list per person with answers per event (*does not suit / poor / good*), an optional own list per task (not built).
- «Even distribution» = **equal total load** (the user's answer 2026-11-09): someone who only takes Video should not end up with half of someone who takes Sound and Video.
  Distribution per task is off by default (`w_task_fair`/`w_task_range` 0). Do not make the per-task term the main term again without asking.
- **The weights are configuration per plan** (2026-11-09): `store.WeightSpecs` (default and limits), stored as what differs from the default, and part of the hash. The defaults must be copies of
  the planner's (`--defaults`; a test checks it). A new weight = one line in `WeightSpecs`, a label and a hint (`weight.<key>.label`/`.hint`) in both languages in `web/i18n.js`, and a group in `WEIGHT_GROUP` in `web/app.js`.

- **Live and without flicker** (2026-11-20, the user's choice: "no loading or flickering", "webhooks for all state", where they meant WebSockets;
  Server-Sent Events kept at their choice ("Behold, men lag mer effektivt"), since the page only needs the push). Every write that went
  through, and the end of a planning run, is announced on the organisation's stream (`GET /api/orgs/{org}/stream`, Server-Sent Events,
  `internal/api/stream.go`; the method and path of the write and the origin tab from `X-Dutyplan-Client`, no data). The page (`follow()` in `web/app.js`)
  keeps what it has fetched (`get()`), forgets only what the write can have changed (`changedBy()`: a new write route needs a line there,
  else everything is fetched again) and redraws in place: scroll (also sideways), focus and open dialogs are kept, a dialog's contents are swapped
  (`keyed()`/`reopened()`, never closed and opened again), and a redraw waits while a text field is being typed in. Clicks on answers,
  levels and ⚡ show at once and go back if the server refuses. Hosting will be behind Cloudflare: the stream pings every 25 s (its idle
  limit is 100 s) and has no write timeout; after 5 failed reconnects in a row (an expired sign-in in front) the page stops and tries again when the tab is shown or after a minute. Do not add polling or full reloads back.
- **Signed out or out of reach** (2026-11-22, the user: "should offer a reauth/refresh instead of showing Object object and failed to fetch"): the
  app's own login answers 401; a sign-in in front of it (forward auth, kate) answers an expired session with a redirect (303). `api()` fetches with `redirect: 'manual'`, so a
  redirect, a 401 or a non-JSON answer means *signed out*, and a network error means *out of reach*; `connection()` then shows a bar at the top with
  «Sign in again» (to `/auth/login?next=<this page>`, or a reload through the sign-in in front) or «Try again», the page stays as it was, a click is taken back, and an open dialog says it too.
  Never show a raw error («Failed to fetch») for these. modal() flattens its parts like h(): `append()` writes a list as «[object …]».
- **Little help text on the page** (2026-11-20): explanations are behind ⓘ buttons (`infoBtn`, or `{ info: [...] }` as modal()'s first argument).
  Every choice with three levels is an icon: red thumb down (can't / no), yellow ? (if needed / maybe), green thumb up (good / yes).
  The main table scrolls inside the window (dates and names stay put); ‹ › / the calendar button (next event from today) / wheel over the dates / arrow keys / dragging move sideways.
- **Folding and less text** (2026-11-22, the user: "collapsible sections and less text"): every table section and every section of people folds
  (`foldBtn`/`folds`/`applyFolds`, rows tagged `data-fold`, remembered per plan in localStorage `fold:<org>/<plan>`). Detail goes into
  hover titles instead of text: the tasks a person takes are the name's title, the draft bar shows `#<id>` (its title the time and
  quality) and a pin toggle, the copy ⧉ shows on hovering its date. The draft's compromises/distribution/earlier drafts sit in its bar.
- **The logo** is `web/logo.svg` (a green rota tile; favicon, header, landing page), drawn 2026-11-22 at the user's request.
- **Assets are versioned** (2026-11-20): the server links `app.js`, `i18n.js` and `style.css` from `index.html` as `?v=<hash of the content>`
  and lets only that exact address be cached (a year, immutable); the page and bare or old addresses are `no-cache`. A new asset must be
  added to `assets` in `internal/api/api.go`. Why: a browser (or Cloudflare's Browser Cache TTL) kept an old `app.js` after a deploy, which
  asked for a removed API and showed «Page not found».

## Languages and texts

- The repo, its docs, code, comments and tests are in **English**. The page is in English and Norwegian (bokmål); the browser's language is the default and the header has a switch.
- **On the wire everything is English and ASCII**: an answer per event `never|poor|good` (blank = not answered), a person's task `poor|good` (no row = not),
  a combination `cannot|practical` (no row = `impractical`, «bad»); error kinds and codes. Texts for people are made at the edge.
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
  Used once so far: the group mutations, when groups were removed (2026-11-20, issue #9).
  History: 2026-11-19: 248 → 256 with the day types and the change log gone; 2026-11-20: 256 → 238 with the per-person API, places set when an event is
  created and cells for several events at once gone; 2026-11-20: 238 → 242 with repeating every few weeks; 242 → 237 with «active» and the choice weight default_combination gone (one click per setting); 237 → 250 with the task order and Fill row; 250 → 272 with the order of people and sections. 2026-11-20: groups removed (see the exception above), then 234 → 247 with the live stream. 2026-11-22: 247 → 274 with signing in and sharing, 274 → 290 with the limits, 290 → 293 with admin/member and links to plans only.
- No real names, e-mail addresses or phone numbers in the repo, not even the church's, the programmes' or the spreadsheet's names, and no known names from the spreadsheet (scrubbed 2026-11-09, and the history was rewritten). Invented names must not be real first names from the original crew: check against the spreadsheet before using new names. Dates and event types in the test data are invented (2026-11-09). All that remains of the shape of the spreadsheet is the task names (common roles such as Sound and Video) and roughly how large the teams are. The test data has invented names and is made by `mise run fixture`.
- The test data is `solver/fixtures/autumn2026.json`, made by `solver/fixtures/make_autumn2026.py` (`mise run fixture`).
- **The pictures in `README.md`** are `docs/img/*.png`, made by `mise run app:shots` (about 20 s, `scripts/shots.js`) from the real app on invented data (the community hall «Maple Hall»; the people are the invented names of `scripts/seed-demo.sh`, so the rule above about names holds). Run it again, and commit the pictures, when the page changes visibly; it removes the old ones first. The day in the pictures is fixed (21 October 2015: the script rewrites the stamps in its temporary database), so a run without a visible change gives byte-identical files. `web/docs_test.go` checks that every picture the README shows exists and that none is left unused (it skips in the mutation copies).
- Do not give a script a name that collides with the standard library (`inspect.py` broke the numpy import).
- Do not run `mise lock` here: `lockfile` in this configuration wrote a `mise.lock` in `~/code` (the parent configuration). The versions are pinned in `mise.toml`.
