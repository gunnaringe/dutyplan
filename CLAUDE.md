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
events at once, and no places when an event is created (adding repeatedly with `until` and `every`, copying and the availability map stay: the page uses them).
**CI** (GitHub Actions, since 2026-11-20): `.github/workflows/ci.yml` runs `mise run lint`, `test`, `app:smoke`, `app:ui` (playwright-core from npm, the runner's
Chrome) and, in a job of its own, the mutations: on a PR only those of the files it changes and those it adds or changes
(`mise run mutations -- --changed-since <base>`, every pattern still checked for OUTDATED; 2026-11-23, the user: "Can we make the ci
job faster? Just check the solver if we touch that one directly?"), on main all of them (about 25 min; merges to main do not cancel
each other's run). A test weakened in a PR is therefore only caught on main: look at main's run after merging. **`main` is protected** (ruleset «main», 2026-11-20): changes only through a PR, the checks `test`, `mutations` and `build` must pass before
it can merge, no force push or deletion; no review is required. Secret scanning with push protection is on. `.github/workflows/image.yml` builds the image
(amd64 + arm64; the Go stage cross-compiles) on PRs and publishes `ghcr.io/gunnaringe/dutyplan` from main (`main`, `main-<UTC>-<sha>`, `sha-<sha>`) and `v*` tags.
A first test runs on kate since 2026-11-20 (asked by the user), behind Pocket ID at dutyplan.haxxor.xyz, manifests in `clusters/kate/dutyplan/`:
kate runs the published image through Flux image automation (`clusters/kate/flux-system/dutyplan-image.yaml`), with a nightly SQLite backup to NFS.

Layout: `cmd/dutyplan` (main), `internal/store` (SQLite, the schema, all data access), `internal/planner` (builds the input from the database,
starts the Python planner, turns the answer into a view with explanations), `internal/api` (the Connect API + serves the frontend), `internal/i18n` (server texts),
`proto/` (the API's schema), `gen/` (Go code made from it by buf, committed),
`web/` (index.html, i18n.js, app.js, style.css, embedded in the binary), `solver/` (Python/CP-SAT), `scripts/` (smoke test, browser check).

## Run cheaply (read this first)

- **Everything goes through mise**: `mise install` (python, uv, go, node, prek), `prek install` (the hooks of `prek.toml`: the checks on commit, `mise run test` before every push), `mise run setup` (Python packages), then `mise tasks`. Never python/uv/pytest/go directly.
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
  (`/o/<org>/p/<plan>`) and in every API call (`org`, `plan`), the server has no «current organisation». Every lookup is scoped by the organisation, and
  a plan/person from another organisation gives 404 (the isolation tests and the `tenant:` mutations guard this).
- **Signing in** (2026-11-22, the user: "Make it use proper oidc ... let people sign up with new organization themselves"): `internal/api/auth.go`,
  OpenID Connect (code + PKCE, go-oidc) against one provider; in production the Pocket ID at id.sermon-studio.app, shared with sermon-studio
  ("User management and login will be the same"). Session = random token in a host-only `__Host-dutyplan` cookie, its SHA-256 in `session`.
  the `procs` table and `gate` in `internal/api/procs.go` are the one place that decides: pages, `Health` and `Me` open; the rest needs a session (401 `signed_out`); an
  organisation's procedures need a member (`membership`: admin or member), 404 for anyone else. **Roles** (2026-11-22, the user: "the roles are org
  admin and org members + edit and view shared link"; migration 004): an admin renames and deletes the organisation, changes members and makes or
  deletes invitations (403 `admin_only` for others); a member does everything else, plans and links to plans included. Whoever
  creates an organisation is its admin; a taken address gets `-2`, `-3` (never a 409 that tells the name exists). `--no-login` (or listening on
  loopback without `--oidc-issuer`) = no login, everyone may do everything: the tests, the scripts and kate (behind its forward-auth). The
  `access:` mutations guard it.
- **Sharing** (2026-11-22, the user: "possible to create shared links that allow access to the plan or org"): `share_link` (003), `internal/api/access.go`.
  A view or edit link is to **one plan** (no links to a whole organisation since 004); `join` is an invitation (opened signed in = member). Opening `/s/<token>` keeps the
  token in the `__Host-dutyplan-links` cookie (up to 20), signed in or not; the gate lets a request through on a link when no membership does. A
  plan link reaches its plan and the organisation's shared parts (people, tasks, sections; the plan page needs them), never the plan list,
  other plans, members or links, and its stream hears nothing of other plans (`grant.sees`). A link never renames or deletes the organisation.
  Members make and delete plan links; a link never makes links (it does not reach `ListLinks`/`CreateLink`); only an admin invites and changes members; anyone may
  leave; the last admin stays. Tokens are stored as they are, so a link can be copied again. `GetOrg` says `role` / `level` / `plan`, which the page uses to show or hide. The `share:` mutations guard it.
- **Answer links, «My answers»** (2026-11-22, the user: "Vi trenger også en visning for én person. Full planlegging av alt blir litt mye for en frivillig å
  forholde seg til, så gjerne en visning der de kan fylle ut når de selv kan. Foreløpig uten noen bruker-management, så samme lenke."): level `answer`
  (migration 009 makes `share_link` again for the CHECK) reads like a view link and writes only `SetAvailability` and
  `SetNote` (`answer` in the procs table; several links in one browser: the widest wins, `linkRank`). The page gives it the
  modes «My answers» (`mePage`) and View: «Who are you?» picks a name (localStorage `me:`, «Not you? Switch»), then their tasks and every date with
  yes / maybe / no (the same again = not answered) and a note. One link for everyone, anyone may answer for anyone: no user management. The `answer:` mutations guard it.
- **Limits** (2026-11-22, for the public signup): caps in `internal/store/limits.go` (`atMost`, code `too_many_<what>` with `max`; checked
  in the transaction that creates), and a rate limit on changes per person or address in `internal/api/ratelimit.go` (only with a login;
  `--client-ip-header` names the proxy's header; `costly` in the procs table marks the dearer creations). Only `/` with a login may be indexed. The
  `limits:` and `index:` mutations guard them.
- **The header** (2026-11-22, the user, "align the design to be similar to sermon studio"): brand, organisation, then round icon links
  to the organisation's **Members** page (`/o/<org>/members`, members with a login) and **Settings** page (`/o/<org>/settings`: rename and
  delete, admins), the language button (`EN`/`NB`, a menu) and the person signed in as initials (a menu with their name and a Sign out
  button). Buttons in the header are one height (`app:ui` checks it). Members and settings are pages, not dialogs.
  The Members page (2026-11-22) has three parts: the members, «Invite a member» and the links to plans (made under Share
  on each plan, listed here to see and delete in one place). A link is a row: an icon for what it gives, its name with «Can view / Can edit /
  Becomes a member», expiry and maker under it, then round icon buttons (share where `navigator.share` exists, copy, delete).
  An invitation is worded as being for a person, never as «a link»: the form asks «Who are you inviting?» (stored as the link's label)
  and the row reads «Invitation for <label>». «Who can use it»: one person (the default; `single_use`, migration 007: the invitation is
  deleted in the same transaction as someone becoming a member with it; a member opening it uses nothing) or anyone who has it.
  Deleting a link, removing a member, leaving and changing one's own role are asked first in the
  page's own dialog (`ask()`).
- **Selecting cells** (2026-11-22, the user: "drag and drop, and unselect selected cells + mark a whole Row. Kinda like you would in a
  spreedsheet… Make sure the ux fits having a big number of volunteers"): on the plan page (not on phones, not with a view link) drag,
  Shift/Ctrl+click, ⇥ at a row's end (a row; ⋮⋮ only drags) and a count (a day) select answers or places; the `.selbar` at the bottom sets them, with keys 1/2/3/Delete/Escape.
  One request per change: `SetAnswers` and `SetPlacesMany` (`internal/store/bulk.go`, at most
  `MaxCellsPerCall`, all or nothing, every person/task of the organisation and every event of the plan, else 404). The `bulk:` mutations
  guard them; `app:ui` checks the selection. A name filter shows with more than 8 people.
- **Notes on answers** (2026-11-22, the user: "notat/kommentar i hver celle for om dagen passer for en person … vises som en flik i hjørnet når du
  klikker eller på hover"): table `answer_note` (migration 005, same keys as `person_event`, so it stays when the answer is cleared), at most
  `MaxNoteRunes`; `SetNote` (empty removes), listed in `ListAnswers` as `notes`. Never in the planner's input.
  The page: `.note-tab` in the cell's corner (amber with a note), hover shows it, a click opens the one `.note-pop`. The `notes:` mutations guard it.
- **Show tasks** (2026-11-22, the user: "tags på personer som viser gul/grønn tag for oppgaver de er villige til å ta? Kanskje gjøre det mulig å
  ekspande for å vise for alle?"): every person row has its tags (`tagsOf`: planned tasks chosen good = green, poor = yellow) under the name, hidden
  until the table has `show-tags`; the «Show tasks» button in the Availability heading toggles it without a redraw, kept in localStorage (`tags`, for all plans).
- **Who is shown** (2026-11-22, the user: "filtrere hvem du ønsker å vise i plan … en avkryssing, så du kan velge et fritt utvalg"): the 👥 button
  under *Availability* opens a dialog with a ✓ per person. Page only: the hidden ids in localStorage (`hide:<org>/<plan>`), so someone new is
  shown; counts, planner and draft still have everyone. Not stored on the server. Also by task (2026-11-23, the user: "be a filter that also can
  filter on tasks"): tasks ticked at the top of the same dialog (`whotasks:<org>/<plan>`) show only people who take one of them, good or poor;
  the button (a funnel) says e.g. «Video · 2 of 4».
- **Space** (2026-11-23, the user: "Use less padding around the about the events field. We want to be more considerate about ensuring we use our
  space efficiently"): the details rows, the task rows and the add lines are slim (less padding, 1.5rem fields, 1.4rem buttons; ui-check holds
  them to one smaller height). Think about height and width when adding anything to the table.
- **The agent guide** (2026-11-22, the user: "I want a llm page to show how to use it. But don't make something others can not make"):
  `web/llms.txt`, served at `/llms.txt`, for agents that *use* a server. Only what any user can do: access by a share link or a login, putting
  a semester in with the existing API, every procedure. No server-only tools (an import command was made and dropped: "I just want it in"), no
  API keys. `TestTheAgentGuideNamesEveryProcedureAndNoOther` fails when a procedure is added or removed without the guide; the edit-link test checks
  the recipe's steps.
- **All writes go through `store.write`** (one transaction). Do not write to the database any other way. There is no change log and no actor: do not add them back without asking.
- **Renaming changes the name only.** The slug is the address (made from the name when the organisation or plan is created; the API takes no slug); links must keep working. Never make a rename change a slug.
- **The API covers only what the page uses** (2026-11-20: "Update api to only cover what the UI uses. We will rather start simple and expand."). Do not add an endpoint or a
  field the page does not call; add it together with the page change that needs it. **A person in a plan is a row, not an account**: no personal links, no per-person pages or endpoints for
  the people who are planned (name, task choices, can be proposed). Accounts are only for those who plan (see *Signing in*); links that share a
  whole organisation or plan are built (see *Sharing*).
- **The API is Connect** (2026-11-23, the user, after the TypeScript analysis: "Would Buf Connect then be a good choice as well?" … "Go ahead"):
  `proto/dutyplan/v1/dutyplan.proto` (five services: Account, Org, Setup, Plan, Draft; one procedure per call the page makes, 1:1 with the old routes,
  none added or merged), `buf.yaml` (lint STANDARD), `buf.gen.yaml`; the Go code in `gen/` is committed (`mise run gen` after changing the
  .proto; `mise run gen:check`, the `gen` hook and CI fail when it is out of date). JSON only, `POST /dutyplan.v1.<Service>/<Method>`:
  protojson with the proto's snake_case names, every field sent, unknown fields refused (`rpc.go`). Ids are `int32` (numbers in JSON;
  int64 would be strings), levels are strings, `org` is field 1 and `plan` field 2 of every request that has them; a field whose absence
  means something is `optional`. The messages mirror the store's JSON, and `out()` turns a store value into one (a field the message lacks
  is an error, so they cannot drift). **Access is one table** (`procs` in `procs.go`: scope, write, admin, self, link, answer, costly, big;
  `TestEveryProcedureHasALine`), applied by `gate`, an HTTP layer **before** Connect decodes the body (it peeks `org`/`plan`/`id`), so a
  stranger gets 404/401, never 400; `requireJSON` caps the body first (64 KB, 512 KB for `big`) and wants `application/json` after the gate.
  The interceptor only announces writes on the stream. The stream stays Server-Sent Events (`GET /api/orgs/{org}/stream`, not Connect: an
  EventSource cannot read Connect). The Go tests still speak the old REST shapes through `legacy()` (`internal/api/legacy_test.go`,
  translated to Connect calls); the page maps its old (method, path) calls through `ROUTES` in `web/app.js`. Both go when the page moves to
  TypeScript with a generated client (the next step of the same plan).
- **Deleting a plan** is refused while a run is in progress. An organisation is deleted only when it has no plans, and the caller confirms with its slug (the API handler
  writes the deletion to the server log). `TestEveryTableWithOrgOrPlanDataIsKnownToTheDeletions` fails when a table is added that the deletions do not know about: update `DeletePlan`/`DeleteOrg` and the test.
- **The event is the unit, not the date.** Events are self-contained: two can fall on the same day. An event can have a start time (and an end time); the planner does not use them, so they must not change the input or hash of a run. It has **no description** (removed 2026-11-22, the user: "let description just be a field. Remove that from header."; migration 006 moved existing ones into a details row «Description» at the top): what is said about an event is a details row. Availability («can I»), combinations, places
  apply to the event. The planner's «dates» are events (the id is arbitrary text).
- **Pin = the light form of publishing.** One finished run per plan can be pinned as the current plan; the page opens with it (else the newest run).
  Pinned runs are never pruned. There is still no approval flow or publishing.
- **One table per plan, one view** (2026-11-19, from the sketches): events across (date, time, copy; **+** adds one), then four sections:
  *Event details* (the plan's rows, `plan_field`/`event_field`: ⋮⋮ drags a row, its name is a field (empty deletes, asked), the pin freezes it (`frozen`, migration 010, shared: sticky under the dates via `--stick` measured by a ResizeObserver), values suggest the row's texts (`suggest`/`rowValues`; another case is saved in the spelling used); no ✎ dialog since 2026-11-23, the user: "Remove the edit meny of about. Renaming can be done directly"; text for people, never sent to the planner, not in the hash),
  *Tasks* (the places per event, typed into the cells; empty = not needed; ⋮⋮ drags a task, in the organisation's order `task.position`, not in the planner's input; organisation-wide task sections (`task_section`, migration 011, 2026-11-23, the user: "Make it possible to have some sections for the tasks as well. As for people, sections is just a UI concept for now"; `store.Tasks`/`store.People` share the section code) are headings like the people's, also in the draft;
  ⇥ selects the row for the bar at the bottom: no «Fill row» since 2026-11-22; ⚡ by the name = `task.planned`, as by a person), the green *Draft allocation* (the run on show merged with what is set by hand), and
  *Availability* (a count per event and one row per person; ⋮⋮ drags a person, `person.position`; «Add a person» lines after those without a section and at the end of each section, `CreatePerson {name, section}`; organisation-wide *sections* (`person_section`)
  are headings with their people below, those without a section first; only placement, never in the planner's input; on a phone too). Clicking a date, task, person or draft place ⋯ opens its dialog; ⚙ *Plan settings* has the name, the
  weighting, the earlier drafts (Show/Pin) and Delete. Do not add separate pages for these again.
  - **The whole width** (2026-11-23, the user: "Vi har stort behov for horisontal plass. Så for planen bør vi ikke ha en container for dette midt på"):
    the plan page has no max-width and only a small side padding (`main:has(.plan-page)`); other pages may stay centred.
  - **Edit | View** (2026-11-23, the user: "Kanskje også en visning som er en penere visning uten redigering eller tilgjengelighet? … Ikke egne lenker for nå,
    bare to moduser for samme data."): a switch in the title bar (`modeSwitch`; `showPage()` in `pagePlan`), kept per plan in localStorage (`mode:`), no
    route of its own. View = events down, tasks across, what each place holds now (`slotsAt`, from the pinned run, else the newest `done` one: never the draft
    picked in Edit), the details; a card per event on a phone, print CSS for the table, «Find a name» (`find:` in localStorage; a search field with
    a list under it, 2026-11-23: "let the name selector be a nice search field"), «Dates across» (`across:`, "show the table with dates on the x axis") and
    «Only the dates <name> has a task» (`only:`, "only include dates where the selected person has a task"). Every cell of an event has `data-ev` (fading). A view-only link starts in View
    («Table» is the other mode). Covered by ui-check only (no Go in it).
  - **On a phone** the same table as on a wide screen, narrower (2026-11-22, the user: "La vertikal visning på mobil vises som vanlig. Den
    løsningen som er i dag er litt forvirrende." — the turned availability of 2026-11-20 is gone; do not bring it back). Names stay put, the days
    scroll sideways; a tap answers. Dragging a selection needs a mouse (a touch scrolls), but ⇥ and a count still select.
  - **No day types.** Each event has its own places (`event_task`); in the planner's input every event is its own type. Events are copied instead (`CopyEvent`:
    dates, or repeated every week or every 2–8 weeks until a date (`every`; only the dates are made, nothing stores the rhythm), with times, details, places and
    names set by hand as chosen; answers and drafts are never copied).
  - **The draft:** a place set by hand (a manual cell that is not locked) has a violet background and a ✎ square (`mark('hand')`); a planner name that was locked is
    a cell with `locked` and shows a lock square (`mark('lock')`, 2026-11-22, the user: "a square with lock"); what the squares mean is in the draft's ⓘ, not on the bar (2026-11-22, the user: the legend was «messy»); «maybe» marks a place given against a ?; an unfilled place says «Not filled · Why?» and the place dialog explains.
  - **Locking the draft** (2026-11-22, the user: "La en planlagt celle som inneholder navn ha en lås-knapp i tillegg til en meny-knapp … Det bør kanskje være enklere å låse plan?"):
    with an edit right every planner name in a draft cell has a lock button (pressed = locked), and ⋯ opens the place dialog; a click on the cell itself does nothing,
    so dragging over draft cells (or ⇥) selects them, with **Lock** / **Unlock** in the bar (a third grid, `draft`). A lock by each task's name in the draft locks
    its row, and a row of locks under the draft (`tr.draft-locks`) a date (`groupLock`; 2026-11-23, the user: "lock a column or a row").
    **Lock all is gone** (2026-11-23, the user: "Maybe remove the lock all? … Maybe a more visible "we are done" button?"): **We're done**
    locks every planner name of the draft on show, pins it and switches to View; a pinned draft with nothing left to lock shows «✓ Done»
    and **Reopen** (unpins, keeps the locks).
    **Undo** (2026-11-23, the user: "a easier way to regret options"): `planState.undo` (per tab and plan, at most 30, not a change log):
    each draft action (`setLocks`, names added/removed in a cell, pin, Remove unlocked, Done, Reopen) pushes its opposite request(s);
    the bar's Undo and Ctrl+Z outside a text field run the newest. Undoing Remove unlocked pins the run that was pinned, else
    `ClearDraftAside`. **The drafts one by one** ("show the different drafts"): ‹ #n › in the bar, and
    **Show changes** (`changes:` in localStorage) marks the planner's names that are new since the finished draft before (`td.changed`,
    `.slot.new`, the old names in the title). One request: `SetLocks`
    (`internal/store/locks.go`; unlocking never removes a name set by hand). The run's snapshot keeps which names were locked (`Kept`, not in the input or hash),
    so an unlocked name stays in the draft as the planner's, and one set by hand that was removed does not.
    **Remove unlocked** ("Kanskje en fjern alle ulåste også?"): asked in `ask()`, then `SetDraftAside` sets `plan.draft_aside` (migration 008) and unpins:
    the page shows no draft, only what is set by hand and locked, until a new run starts or a draft is pinned (both clear it); the runs stay under the earlier drafts.
    The selection («only these can be picked») stays ("Behold mulighet til å sette utvalg") but is one line in the place dialog, «The planner can pick: … · Change»,
    and a limited place has a small funnel (`mark pool`) in the draft. The `locks:` and `draft:` mutations guard it.
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
- **What matters most** (2026-11-23, the user: "simple settings for what is important. Setting the numbers manually could be hidden
  as a advanced setting … a link … on the plan page directly"): a dialog (`prioritiesDialog`, from the draft's bar and a line in ⚙) with
  three priorities, Less / Normal / More (`PRIORITIES` in `web/app.js`: each level is fixed values for a group of weights, Normal = the
  defaults; numbers set by hand show as Custom), the rules that always hold, and the numbers under «Advanced». Page only: the same PUT
  weights. A dialog, not a page, following «dialogs for the rest» below. If a default weight changes, change Normal with it.
- **The weights are configuration per plan** (2026-11-09): `store.WeightSpecs` (default and limits), stored as what differs from the default, and part of the hash. The defaults must be copies of
  the planner's (`--defaults`; a test checks it). A new weight = one line in `WeightSpecs`, a label and a hint (`weight.<key>.label`/`.hint`) in both languages in `web/i18n.js`, and a group in `WEIGHT_GROUP` in `web/app.js`.

- **Live and without flicker** (2026-11-20, the user's choice: "no loading or flickering", "webhooks for all state", where they meant WebSockets;
  Server-Sent Events kept at their choice ("Behold, men lag mer effektivt"), since the page only needs the push). Every write that went
  through, and the end of a planning run, is announced on the organisation's stream (`GET /api/orgs/{org}/stream`, Server-Sent Events,
  `internal/api/stream.go`; the procedure of the write (`SetPlaces`; `RunFinished`, `Joined`), its plan and the origin tab from `X-Dutyplan-Client`, no data). The page (`follow()` in `web/app.js`)
  keeps what it has fetched (`get()`), forgets only what the write can have changed (`changedBy()`: a new write procedure needs a line in `CHANGES` there,
  else everything is fetched again) and redraws in place: scroll (also sideways), focus and open dialogs are kept, a dialog's contents are swapped
  (`keyed()`/`reopened()`, never closed and opened again), and a redraw waits while a text field is being typed in. Clicks on answers,
  levels and ⚡ show at once and go back if the server refuses. Hosting will be behind Cloudflare: the stream pings every 25 s (its idle
  limit is 100 s) and has no write timeout; after 5 failed reconnects in a row (an expired sign-in in front) the page stops and tries again when the tab is shown or after a minute. Do not add polling or full reloads back.
- **Signed out or out of reach** (2026-11-22, the user: "should offer a reauth/refresh instead of showing Object object and failed to fetch"): the
  app's own login answers 401; a sign-in in front of it (forward auth, kate) answers an expired session with a redirect (303). `api()` fetches with `redirect: 'manual'`, so a
  redirect, a 401 or a non-JSON answer means *signed out*, and a network error means *out of reach*; `connection()` then shows a bar at the top with
  «Sign in again» (to `/auth/login?next=<this page>`, or a reload through the sign-in in front) or «Try again», the page stays as it was, a click is taken back, and an open dialog says it too.
  Never show a raw error («Failed to fetch») for these. modal() flattens its parts like h(): `append()` writes a list as «[object …]».
- **No dialogs of the browser** (2026-11-22, the user: "La alle dialoger og bekreftelser være pent rendret."): never `confirm()`, `prompt()` or
  `alert()`. Ask with `ask()` (or `await confirmed(title, text, yes)` inside a `write()` action, where «no» ends it quietly); a link
  without a clipboard is shown selected in a `modal()`. `web/dialogs_test.go` fails on a native dialog; ui-check counts one as an error.
- **Little help text on the page** (2026-11-20): explanations are behind ⓘ buttons (`infoBtn`, or `{ info: [...] }` as modal()'s first argument).
  Every choice with three levels is an icon: red thumb down (can't / no), yellow ? (if needed / maybe), green thumb up (good / yes).
  The main table scrolls inside the window (dates and names stay put); ‹ › / the calendar button (next event from today) / wheel over the dates / arrow keys / dragging move sideways.
- **Every other date is shaded** (2026-11-23, the user: "alternativt background colors to aid navigation"): `--alt` (a light blue since 2026-11-23,
  the user: "more colorful. Maybe light green? Or light blue?"; green is taken by the draft) as a background-image on even columns (thead th,
  tbody td without colspan), so the cells' own colours stay. A details value fills its cell with rounded corners and lets the shade through.
- **Redraws must stay cheap** (2026-11-23, the user: lag when setting a person's tasks): a change redraws the whole table, so anything done
  per cell counts. Dates go through cached `Intl.DateTimeFormat`s (`formatted`): `toLocaleDateString` per cell was over half of a redraw
  with 50 people. Level buttons (`cycle`) never drop a quick click; one save at a time, the last level shown is what is saved.
- **Moving rows and the add lines** (2026-11-23, the user: "Let the drag and drop of people easier. Hide the add people icon under a small
  plus symbol at the end of the last cells, where clicking it makes it a minus and collapse it"): rows move by pointer events, not HTML
  drag (`onRowDown` on the table; `handleFor` marks ⋮⋮, `dropOn(row, accept, act(item, after, kind), into)`): with a mouse the whole name
  cell is the handle (5 px before it starts, the click after is swallowed), on touch only ⋮⋮; a `.drop-line` above/below by the row's
  half, a heading (`into`) puts it last; autoscroll near the edges; Escape lets go. The add lines are behind `adds(key, rows, host, lines)`:
  a `button.addplus` in a strip at the bottom of the last name cell (the heading when empty), open keys in `planState.adds`,
  `applyFolds` hides the closed ones; the last list of people also opens «Add a section». Section names are typed in place (`nameField`).
- **Names in a draft cell** (2026-11-23, the user: "click a planning cell to easily add names. Show autocompletions. Adding a name that is not a
  person makes it a custom name. Add a plus … We allow adding more people then we have spots", and "No need to warn if adding more people then
  required"): `editCell` replaces the cell with chips + a combobox (`input.ename`, `ul.elist`); a plain click or + opens it (Ctrl/Shift
  clicks and drags still select). The custom name is an explicit last option; a name the planner put there is posted `locked`; × only on
  names set by hand. The field opens again after the redraw (`planState.edit`/`editInput`, typed text kept). Never a count or warning over the places.
- **Folding and less text** (2026-11-22, the user: "collapsible sections and less text"): every table section and every section of people folds
  (`foldBtn`/`folds`/`applyFolds`, rows tagged `data-fold`, remembered per plan in localStorage `fold:<org>/<plan>`). Detail goes into
  hover titles instead of text: the tasks a person takes are the name's title, the draft bar shows `#<id>` (its title the time and
  quality) and a pin toggle, the copy button (`icon('copy')`) shows on hovering its date; its dialog suggests the first free date in the rhythm, lists the dates it makes (`ul.copy-dates`, one with an event marked) and says «Make n copies»; «Places» is «People needed» there (2026-11-23, the user: "Improve the copy button for events"). The draft's compromises/distribution/earlier drafts sit in its bar.
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
- **Errors** are `store.Invalid(code, params, englishMessage)` / `store.Conflict(...)`; the answer is a Connect error with a `dutyplan.v1.Problem` detail `{error, code, params}` (`fail()` in `internal/api/rpc.go`; the page reads `details[0].debug`). A new code needs `err.<code>` in both catalogues of `web/i18n.js`.
- Page addresses are English: `/o/<org>` and `/o/<org>/p/<plan>`; anything else is «Page not found».

## Testing and data

- `check.py` is an independent checker; keep it independent of `model.py` (only the default values are shared).
- Every rule must have a test that fails when the rule is broken in the model: `solver/scripts/mutations.py` (`PY_MUTATIONS`, `GO_MUTATIONS`) checks it. An outdated pattern (`OUTDATED`) is fixed, never deleted, and the number of mutations must not drop.
  **The one exception** (the user's choice, 2026-11-20): a mutation may be deleted only when **the user asked for the feature it guards to be removed**, and the
  commit message names the mutations and that request. A pattern that is merely hard to fix, or a rule you think is no longer needed, is not a reason.
  Used once so far: the group mutations, when groups were removed (2026-11-20, issue #9).
  History: 2026-11-19: 248 → 256 with the day types and the change log gone; 2026-11-20: 256 → 238 with the per-person API, places set when an event is
  created and cells for several events at once gone; 2026-11-20: 238 → 242 with repeating every few weeks; 242 → 237 with «active» and the choice weight default_combination gone (one click per setting); 237 → 250 with the task order and Fill row; 250 → 272 with the order of people and sections. 2026-11-20: groups removed (see the exception above), then 234 → 247 with the live stream. 2026-11-22: 247 → 274 with signing in and sharing, 274 → 290 with the limits, 290 → 293 with admin/member and links to plans only, 293 → 302 with selecting cells, 302 → 309 with notes on answers, 309 → 311 with the agent guide, 311 → 310 with the event description removed (4 deleted at the user's request, 3 for migration 006); 310 → 306 with Fill row removed (its mutations deleted at the user's request: "Istedenfor å la en oppgave ha en fyll raden"), 306 → 320 with locking the draft and Remove unlocked. 2026-11-23: 320 → 327 with answer links, 327 → 331 with frozen details rows, 331 → 339 with task sections, 339 → 342 with undoing Remove unlocked.
- No real names, e-mail addresses or phone numbers in the repo, not even the church's, the programmes' or the spreadsheet's names, and no known names from the spreadsheet (scrubbed 2026-11-09, and the history was rewritten). Invented names must not be real first names from the original crew: check against the spreadsheet before using new names. Dates and event types in the test data are invented (2026-11-09). All that remains of the shape of the spreadsheet is the task names (common roles such as Sound and Video) and roughly how large the teams are. The test data has invented names and is made by `mise run fixture`.
- The test data is `solver/fixtures/autumn2026.json`, made by `solver/fixtures/make_autumn2026.py` (`mise run fixture`).
- **The signature** in the start page's About is `web/signature.woff2`, the author's handwriting cut down to the letters of the name (copied from
  sermon-studio, 2026-11-22, at the user's request). Not under the licence (`NOTICE`): never add letters to it or use it for anything else.
- **The film on the start page** is `web/demo-en.mp4` and `web/demo-nb.mp4` (embedded, served with an ETag and ranges), made by
  `mise run app:video [en|nb]` (about 3 min, `scripts/video.js`; ffmpeg comes with the task, not in CI): the real app on the same invented
  data, recorded with the CDP screencast (Playwright's own recorder needs a separate ffmpeg download), a drawn pointer and a caption per
  step, the planner's wait cut out, played at 1.4×, H.264 at 1024×576 (about 400 KB each: keep each under the 500 KB large-file hook).
  Run it again when what it shows changes visibly (adding events, places, answers, the draft, a place set by hand); the selectors are by
  structure and by the texts in `web/i18n.js`, so a changed text there is picked up. Re-encoding never gives identical bytes; commit the
  files only when the film changed. The start page plays it muted and looping, and still with controls under `prefers-reduced-motion`.
- **The pictures in `README.md`** are `docs/img/*.png`, made by `mise run app:shots` (about 20 s, `scripts/shots.js`) from the real app on invented data (the community hall «Maple Hall»; the people are the invented names of `scripts/seed-demo.sh`, so the rule above about names holds). Run it again, and commit the pictures, when the page changes visibly; it removes the old ones first. The day in the pictures is fixed (21 October 2015: the script rewrites the stamps in its temporary database), so a run without a visible change gives byte-identical files. `web/docs_test.go` checks that every picture the README shows exists and that none is left unused (it skips in the mutation copies).
- Do not give a script a name that collides with the standard library (`inspect.py` broke the numpy import).
- Do not run `mise lock` here: `lockfile` in this configuration wrote a `mise.lock` in `~/code` (the parent configuration). The versions are pinned in `mise.toml`.
