# Dutyplan: plan

Working title (*Tjenesteplan* in Norwegian). Status 2026-11-09: **the planner (`solver/`) is connected to the app; phases 1–5 are built (see *Phases*), plus run history with a
pinned current plan, renaming and deleting, and an English/Norwegian page. Deployment is postponed. Private GitHub repo `gunnaringe/dutyplan`.**
First use and test data: a church's rota for autumn 2026 (invented names in the repo). The tool is general; see *General*.

## Goal

Share duties (sound, video, chairing, stewards, … anything) among the events in a period so that

1. **hard requirements are never broken** (does not suit, cannot be combined, manual selection),
2. **preferences are followed as well as possible**, and
3. **the load is as equal as possible**.

People say themselves when it suits them. The planner makes a *proposal*; whatever is entered by hand (for example that a
choir sings at a particular event) is *locked* and never moved.

## General

The tool is general. **Everything is data that the admin creates**: tasks, people, groups (teams), events and their places and details, and the
settings between them. Nothing in the planner, the data model or the wording of the app is tied to a church. Names such as Sound,
Choir, Service and Alpha exist only in the test data and as examples here. The page is in English and Norwegian (bokmål); the
texts are in `web/i18n.js` and `internal/i18n`, and more languages are a catalogue each.

## Scope of the first version

- **No login, no links with a secret, no roles.** See *Access without a login*.
- **Groups are general.** A group is a named set of people (a team, a choir, a couple, a crew). No group has special rules. The admin can put a
  group in a manual cell, and **the planner can also propose groups** as candidates for the tasks the admin has set up for the group
  (`group_task`: which task, how many places it fills, and a level). In both cases the members are busy with the task, and the shared
  combination setting decides what else they can have the same day (see *Manual cells, groups and selections*).
- **Group work counts as one assignment per member** in the distribution (overall and per task), for manual and proposed groups alike.
- **No spreadsheet support**: neither import from nor export to spreadsheets (decided 2026-11-08). Everything is entered in the app.
- No skills and no conditional requirements ("when X, then Y"). Selections cover that.
- No upper limit on the number of tasks per person per day, and no rest requirement between events. The combination setting alone
  governs what can, should and should not happen on the same day. (Both were tried and removed at the user's request, 2026-11-04.)

## How it is used

**Anyone with the address**
- Opens the page and picks their own name.
- **Chooses which tasks to take part in.** All tasks are hidden at first; "choose tasks" lets the person tick more. What is not ticked is not shown and is
  never assigned personally (except through a group the admin has set up for the task, see *Manual cells, groups and selections*).
- Chooses a preference per task: *never / poor / neutral / good*. People differ, and it is up to them.
- Chooses for each **event**: *does not suit / poor / good* (not marked = neutral). **One list covers all the tasks the person takes part in.**
  Sound and Video share the preference, so there is one choice per event. The event counts, not the calendar day: two events on the same day
  have an answer each. Since 2026-11-13 this is their row in the plan's table (see *The table*): a click cycles blank → good → poor → does not suit → blank.
- A task with a different pattern can get **its own list** (off by default; not built yet). It starts as a copy of the shared one.
- Sees the plan in the same table (the draft rows show who got what). There are no user accounts and no personal links: a person is a row
  (2026-11-20: no individual user management for now).

**Setup and planning** (part of the same app, not a separate area)
- Tasks, people, groups, events with their own places, **a shared setting for which tasks can be combined**, places set by hand and selections. Running the
  planner, reading the explanations, locking proposals, looking at earlier runs and pinning one as the current plan.

**Manual cells, groups and selections** (two general tools that cover "the choir sings", "their own sound engineer" and everything like it)
- The admin can enter a **manual cell** for a task and an event: a person, a **group** or a text (for example "Outside guest"). It is locked and counts as a
  filled place. A group or a text can fill several places (`fills`).
- A **group** in a manual cell makes the members busy with that task that day. What they can have in addition is governed by the shared combination
  setting, with no special rule. Example: the group "Choir" is on the task "Singing" on 20 Sept, and *Singing × Sound* and *Singing × Video* are set to
  "cannot be combined". Then the members are out of Sound and Video that day. If the pair is "impractical" or "practical" instead, it becomes a penalty or a bonus, not a ban.
- The admin can set a **selection** for a task and an event: only these can be picked. The planner chooses among them. "Their own sound engineer when the choir sings" is a
  selection for Sound of two people who take part in the task and do not sing. If the selection is as large as the number of places, it is in effect a manual assignment.
- A selection is an ordinary hard requirement. If someone in the selection has "does not suit" that day or does not take part in the task, that comes out as an
  explanation (and an empty place if nobody in the selection can).
- If a group member has "does not suit" on the event a manual group is set for, the manual choice is kept, with a warning.
- **Proposed groups, what the admin has to set up:** for each group which tasks it may be proposed for, how many places it fills, and a level. One
  member with "does not suit" on an event rules the whole group out for that event; a couple is not usable on an event one of them cannot make.
- **Can be proposed, per person and per group** (default yes). When it is no, the planner never picks the person or group itself (not through a selection
  either), but the admin can still enter them by hand. A group is not proposed either if a member cannot be proposed. The group is proposed for the tasks the admin has
  set up for it, whether or not the members ticked the task themselves; this setting governs it.
- **Proposed groups:** the admin sets up which tasks a group can be proposed for (`group_task`). The group is a candidate on a par with a person and fills `fills`
  places. It is ruled out of an event if one member has "does not suit" (or has said "never" to the task), if the group is outside a selection, or if the
  members are busy through another group. The points for the event are the average of the members' wishes; the task level is set on the group. The members' own
  participation in the task is not needed. A selection can name both people and groups.
- Two groups with a member in common cannot both be picked on the same day if it is the same task or "cannot be combined".

## Data model (SQLite)

**An organisation has several plans.** `organization` → `plan` (for example "Autumn 2026", "Spring 2027"). What is shared by an organisation is shared by
all its plans; what belongs to a period lies under the plan.

*Built (one schema file, `001_schema.sql`, since 2026-11-19; not live, so the earlier migrations were squashed and an old database must be deleted):* `organization`, `plan`, `person`, `task`, `task_combination`, `event`, `event_task`, `plan_field`, `event_field`, `person_task`, `person_event`, `plan_run`, `grp`, `grp_member`, `grp_task`, `manual_cell`, `pool_member`.
Every child table has `org_id` and composite foreign keys to `(org_id, id)`, so the database itself refuses references across organisations.
Migrations are forward-only and run in a transaction at startup.

| Table | Level | Contents |
|---|---|---|
| `organization` | | `slug` (unique, in the address), name |
| `plan` | organisation | `slug` (unique in the organisation), name, **`weights`** (JSON: the planner weights that differ from the default). Replaces "period/semester" |
| `person` | organisation | name (unique in the organisation), **can be proposed** (default yes). Reused in all plans |
| `group`, `group_member` | organisation | **built as `grp`, `grp_member`**: a named set of people (team, choir, couple). No rules of its own. **Can be proposed** (default yes) |
| `group_task` | organisation | **built as `grp_task`**: the tasks the planner may propose the group for: places it fills (`fills`) and a level |
| `task` | organisation | name, whether the planner distributes it (otherwise only for manual cells and combinations) |
| `task_combination` | organisation | **shared**: task A × task B: *cannot be combined / impractical / practical / should be combined* |
| `person_task` | organisation | **built**: the person takes part in the task (their own choice; no row = not taking part) and a level (never / poor / neutral / good). Only tasks the planner distributes can be chosen |
| `event` | plan | **event**: date, times, description. A self-contained unit; several on the same day are allowed |
| `event_task` | plan | the places a task has on an event (no row = not needed) |
| `plan_field`, `event_field` | plan | the plan's details rows (a note, a theme; ordered by `position`) and their text per event. For people only: never in the input or hash |
| `person_event` | plan | **built**: level per event (does not suit / poor / good; no row = neutral), shared by the person's tasks. Composite foreign key (org, plan, event) |
| `person_task_event` | plan | *not built yet*: added when a task has "its own list"; it is then used instead of the shared one |
| `pool_member` | plan | **built**: the **selection** for one task on one event: the only people and groups that can be picked (if there are rows, there is a selection). Composite foreign key to (org, plan, event) |
| `manual_cell` | plan | **built**: a place set by hand for a task on an event: one person, one group or a text (+ `fills` places for a group and a text), or (`locked`) a planner name that was locked. Both are fixed for the planner. Proposals from the planner are not stored here but in `plan_run` |
| `plan_run` | plan | status (running/done/failed), seed, time limit, hash of the input, the input, a snapshot of names and places, the planner's answer, error message, **`pinned`** (one per plan, partial unique index). The 20 newest per plan are kept, and the pinned one always |

**Optimisation happens within one plan only** (the user's choice 2026-11-09). Assignments in earlier plans do not count: the app never sends `history` or `previous_plan` to the planner (it can still accept them).
Every lookup is scoped by the organisation; a plan or person from another organisation gives 404.

**Events:** an event belongs to one plan and has a date (and a time of day, a description, its own places and the details rows' text). Events are **self-contained**: two can fall on the same day (for example a
service and a staff evening, or two shifts), and availability, combinations, places and group busyness apply to the event, never to the calendar day. The API uses the id (`/events/{id}`).
A new event can repeat **every week, or every 2 to 8 weeks, until** a date (at most 366 days; added 2026-11-20 at the user's request: «every other week and such»);
**copying** an event (`POST /events/{id}/copy`) makes it again on other dates, or repeated the same way until a date,
with the times, the details, the places and the names set by hand as chosen. Answers and drafts are never copied. There are no day types (removed 2026-11-19): in the planner's
input every event is its own type with its own places.
**Time of day and description (built 2026-11-09):** `start_time` and an optional `end_time` are `HH:MM` on a 24-hour clock (local time, no time zone; the end is after the start on the same day, and an end needs a start);
`description` is free text up to 2000 characters with line breaks. Within a date the events with a start time come first in time order, the rest in the order of creation.
The planner does not use the times: its input lists the events by date and creation, so setting a time never makes a run out of date. The plan shows the current time and description of an event while it exists (else the ones from when the run was made).
Deleting a task removes its places and combinations; it is refused while it is used in a place set by hand (409).

### The same scale everywhere
An answer per event uses **never / poor / good** (blank = not answered). A person's task is **not / poor / good** (not = no row), and combinations are
**cannot / impractical / practical** where no row is impractical (2026-11-20: one click per setting; *neutral* tasks and *preferred* pairs are gone). These are also the codes on the wire.

## The planner (CP-SAT)

One yes/no variable per (person, task, event) that is possible after filtering. Typically 1–2 thousand variables.

**Hard requirements**
- "never" on an event or a task removes the variable. So does "not taking part in the task".
- Selections (`pool_member`): everyone outside the selection is removed for that task and event.
- The number of places per task and event (locked cells count).
- Task pairs that *cannot be combined*: at most one of them per person per day.
- A group in a manual cell or proposed by the planner: each member is busy with the group's task. The pair (the group's task,
  the member's other task) follows the combination setting (ban, penalty or bonus), and the member cannot take an extra
  place on the same task.

**Objective (points, tuned against the test data)**
- Plus for "good", minus for "poor", on event and task.
- Plus/minus for *practical*, *impractical* and *should be combined*.
- **Even distribution = equal total load (the main term; decided by the user 2026-11-09).** Each person's target is their share of all assignments (group work
  counts one assignment per member), computed from how many events the person can make. The target is rarely a whole number, so a **band**
  [lower, upper whole number] is "even enough", and only what falls outside the band costs (`w_fair`, 10). The worst deviation is penalised as well (`w_fair_range`, 10), so that one
  person does not take all the deviation. Which tasks the assignments are in does not matter: someone who only takes Video should not end up with half of what someone who takes Sound and Video gets.
  (The target is based on the number of places; a proposed group with more members than places gives more assignments than places, and that is not counted into the target beforehand.)
- **Distribution per task is off by default** (`w_task_fair` and `w_task_range` are 0, and then no variables are built for it). When it is on, everyone who can do a
  task gets an equal share of exactly that task, and people with more tasks then get more in total. The price of total-first: the same task can be shared unevenly among
  those who can do it (see *Results from the spike*).
- **History from earlier plans is not used** (decision 2026-11-09). The planner can accept `history` per task (counts half, `history_weight`),
  but the app does not send it.
- On a new run the planner can penalise changes from a previous plan (`previous_plan`), but the app does not use it. What should stay is locked as a manual cell ("Lock" on a proposal).
- An empty place is allowed, but costs so much that the planner only does it when it cannot be avoided.

**Explanations**: for every empty place or broken "poor" a short text is made, for example "Task X 15 Nov: 4 candidates,
2 have *does not suit*, 3 are outside the selection, 1 is busy through a group". This is a separate simple filter over the input, not an infeasibility analysis. The server writes
the texts in the page's language (`?lang=`) from codes and arguments kept in the run, so one run can be read in both languages.

## Architecture: a proper backend

- **A Go binary with embedded SQLite** (`modernc.org/sqlite`, pure Go, no cgo), numbered migrations embedded and run in a
  transaction at startup, and a vanilla JS frontend embedded. Runs on kate as a first test (2026-11-20; see phase 6).
  A backend is needed for shared storage between all users, for the planner and for the history, even without a login.
- **API: plain JSON over `net/http`, not Connect** (chosen 2026-11-07, differs from `scoreboard/`). There are well under ten kinds of endpoint, and
  Connect would have needed `buf`, two code generators and generated code. Switching later is cheap because the interface is narrow.
  Errors are `{"error": kind, "code": code, "params": {...}, "message": english text}` with the right status; the page writes its own text from the code and the parameters.
- **The organisation is in the address**, in the page (`/o/<org>`, `/o/<org>/p/<plan>`) and the API (`/api/orgs/{org}/...`). The server has no
  "current organisation", so switching is just going to another address. The list of organisations can be read by anyone who reaches the address.
- **Renaming changes the name only**; the address stays, so links keep working.
- **The API covers only what the page uses** (2026-11-20: "start simple and expand"). An endpoint or a field the page does not call is not kept "for later";
  it is added with the page change that needs it.
- **Foreign keys** are off by default in SQLite on every connection and are switched on in the DSN; a test checks that they are enforced.
- **The solver is a Python program with `ortools`** that Go starts as a subprocess: JSON in on stdin, JSON out on stdout, with a time
  limit. The same container image. The contract is small and can be tested without the rest of the app. **Python is kept** (the user's answer
  2026-11-08: "if Python is better suited it can be kept"). The reason is that it is the same CP-SAT solver, the `ortools` pip package ships prebuilt
  with the native libraries, and the model has its own tests and mutations for every rule.
- **Go alternatives, investigated 2026-11-08:**
  - *The official Go API for CP-SAT* (`github.com/google/or-tools/ortools/sat/go/cpmodel`): exists, but is a cgo shell around OR-Tools' C API. It has no version
    tags (a pseudo-version from October 2026), no known users, and is built with Bazel against the native library, not as an ordinary Go dependency. The app would
    lose "one static binary" and get a heavy build chain, and the model and all the tests would have to be ported. Usable later if Python becomes a problem.
  - *Unofficial cgo shell* (`irfansharif/solver`): bound to OR-Tools 9.1, last changed April 2023. Not an option.
  - *Pure Go constraint libraries* (for example `centipede`, `go-constraint-solver`): solve satisfiability by backtracking, not optimisation with even distribution. Not equivalent.
  - *MIP solvers through cgo* (for example HiGHS): also need a native library, and are weaker than CP-SAT on this kind of problem.
  - I found no pure Go solver that can do what CP-SAT does here.

### Running the planner (built, phase 4 round 1)
- `POST /api/orgs/{org}/plans/{plan}/runs` (`{time_limit}`, optional; the server picks the seed) answers 202 with an id at once; the run goes on in the background and the page
  asks `GET .../runs/{id|latest}` every second. Only **one run at a time** in the whole server (409 `busy`), time limit 5–120 s (default 20).
  With no planner configured (`--solver` / `$DUTYPLAN_SOLVER`) the API answers 503 and `/api/health` has `"planner": false`; the rest of the app works.
- Go builds the input from the database (`internal/planner/input.go`): everyone in the organisation, the ids `t<id>/e<id>/d<id>/p<id>`, "can be proposed",
  task choices, availability per event and combinations. The hash of the input (without seed and time) is stored, so an old plan can be marked **out of date**
  when answers or setup have changed since.
- The planner is started with `--check`: the independent checker goes over the answer and `violations` are stored and shown. An answer that is not valid JSON with status
  OPTIMAL/FEASIBLE, a timeout and a crash become a **failed run with a reason**, never half a plan. Runs that were "running" at startup are marked failed.
- The view (`internal/planner/view.go`) puts names and tasks from the snapshot and gives every empty place an explanation in the page's language
  (`explain.go`: does not suit, has not chosen the task, cannot be proposed, busy with something else). "Try again" counts the seed up.
- **Earlier runs and the pinned plan:** `GET .../runs` lists the newest runs and the pinned one; `PUT|DELETE .../runs/{id}/pin` pins or unpins a finished run (only one per plan; pinned runs
  are never pruned). The page shows any run and pins the one on show. This is the light form of "publishing": a person does not have to approve each proposal, but the admin can say which run people see.

### Weighting per plan (built, after phase 4 round 2)
- The planner's weights are settings on the plan (`GET|PUT /plans/{plan}/weights`, the "Weighting" section of the plan). The ones that can be changed are in `store.WeightSpecs`
  with default and limits (distribution overall and per task, points for "good" and "poor" on task and event, points for combinations and what a pair with no setting counts as).
  Not included: the price of an empty place (must come after everything else), stability and history (the app does not use them) and neutral (zero).
- Only what differs from the default is stored (`plan.weights`, JSON), so a changed default reaches plans that never chose anything. Values outside the limits and unknown keys are refused;
  the sign follows the meaning (a "poor" cannot give plus). A plan set fully back has the same input and hash as before weighting existed.
- The weights go to the planner as `params` (keys with a dot, such as `w_task.good`, are nested) and are part of the hash: a different weighting gives a different plan, so an old run is marked out of date.
  Time, seed and threads are not. The defaults in Go are copies of the planner's own, and a test against the real planner (`--defaults`) keeps them equal.
- Per plan, not per organisation: the plan is the unit that is optimised within. If new plans should inherit from the previous one or from an organisation default, that is added then.

### Grid (built, phase 5; replaced by *The table*)
- The draft rows use the same view as the list of runs (`/runs/{id}`); the explanation for an empty place is behind "Why?".
  «My assignments» per person was removed on 2026-11-20 with the rest of the per-person API.

### The table (built 2026-11-13, redone from the sketches 2026-11-19)
One view per plan, inspired by *whenever*: the person making the plan sets everything in the same table people give their answers in. Read a column from the top:
what the event is, what it needs, who got it, who can come.
- **Columns are events**: the date and times (click: the event's dialog with times, the details, description, delete, copy), ⧉ copies it (to dates, or every week or every few weeks until a
  date, with times, details, places and names set by hand as chosen), and "+" after the last adds one (empty or as a copy, also repeated).
- **Event details**: the plan's own rows (`plan_field` + `event_field`; for example a note or a theme), typed straight into the cells; text for people, not for the planner
  (not in the input or hash). ⋮⋮ drags a row (`PUT /fields/{id}/position`), ✎ renames, moves or deletes it. `GET|POST /plans/{plan}/fields`, `PATCH|DELETE /fields/{id}`,
  `PUT /events/{id}/fields/{field}`.
- **Tasks**: one row per task with the places per event, typed in (`event_task`, `PUT|DELETE /events/{id}/places/{task}`, `GET /plans/{plan}/places`); empty = not needed.
  *Fill row* (`PUT /plans/{plan}/tasks/{id}/places {slots}`, one transaction, 0 clears) sets every event of the plan. ⋮⋮ drags a task (`task.position`,
  the organisation's order, `PUT /tasks/{id}/position`); the planner's input keeps the tasks by id, so the order is not in the hash.
  Clicking the task: rename, planned or only by hand, "Cannot be combined with anything" (`PUT /orgs/{org}/tasks/{id}/combinations`) and the single pairs, delete.
- **Draft allocation** (green): the bar has "Make a draft"/"Try again", the time limit, "Draft #n · time", "✓ Current plan" with Unpin (or Pin), and a key. One row per task:
  what is set by hand (violet), a locked planner name («locked» chip), the planner's names («maybe» when given against a ?), and "Not filled · Why?". Clicking a place opens
  "Place in the draft": remove/unlock what is set, lock a planner name, set a person, a group or a text by hand, and the selection (whom the planner picks from).
  Compromises, the distribution and earlier drafts are links under the rows. The page opens with the pinned draft, else the newest.
- **Availability**: per event how many said ✓, ? and ✗; then one row per person (`GET /plans/{plan}/answers` loads everyone's): blank = not answered, a click cycles
  ✓ → ? → ✗ → blank. Clicking the name: tasks and levels, rename, can be proposed, section. ⋮⋮ drags a person (`person.position`, per section,
  `PUT /people/{id}/position {section, position}`). *Sections* (`person_section`, organisation-wide, `/sections`) are headings with their people below;
  those without one come first, deleting one puts its people last of those. Only placement: the planner's input keeps people by id.
- **⚙ Plan settings**: the name, the weighting, the earlier drafts (Show, Pin) and Delete.
- **On a phone** (option A of the sketch, chosen 2026-11-20): *Availability* is turned and comes first: events down (date, time, the count), people across (first
  names, a tap on one opens the person), and under each answer the tasks the person has in the draft (set by hand, locked or proposed; through a group too). The
  details, places and the draft follow in the ordinary table, which scrolls sideways under the row names. The page draws one layout at a time and switches when
  the width crosses 40rem.
- The organisation page has the plans and the groups, and rename/delete of the organisation. Any other address (an old setup page, an old personal
  link) is "Page not found".

### Groups, manual cells and selections (built, phase 4 round 2)
- **Groups** (`/orgs/{org}/groups`, on the organisation page): name, members, "can be proposed" and the tasks the planner may propose the group for
  (places and level; only tasks the planner distributes). Not deleted while a cell or selection names it (409).
- **Manual cells** (`/plans/{plan}/cells`) and **selections** (`/plans/{plan}/pools`): one event at a time, only an event in the plan (copying an event can take its cells set by hand along).
  A cell can be on a task the planner does not distribute (for example "Singing" for the choir). "Lock" on a planner name makes a cell
  with `locked` for it, which the planner keeps like any other cell.
- **Groups without members** are never sent to the planner (it would count the place without knowing who). What names them is left out,
  and the run says so ("The cell for the group «Choir» on Sound … was left out"). A selection that becomes empty is kept (nobody can be picked) with a warning.
  There is no «active» flag (removed 2026-11-20).
- The planner's warnings are read with names; a cell for a person who "cannot be proposed" gives no warning (that is how the setting is meant to be used).

### Renaming and deleting (built)
- Organisations and plans can be renamed (`PATCH`), people and tasks too (they already could be); the address (slug) never changes.
- **A plan is deleted** (`DELETE`) with its events, places, details rows, answers, manual cells, selections and runs (a pinned one too). Refused while a run is in progress (409 `plan_busy`).
- **An organisation is deleted** (`DELETE ...?confirm=<slug>`) only when it has no plans (409 `org_has_plans`), with its people, tasks, groups and combinations. The API writes the
  deletion to the server log.
- A test lists the tables with `org_id` and `plan_id`, so a new table cannot be forgotten by the deletions; another checks that nothing is left after a deletion.

### Access without a login
The first version has no login, no personal links and no roles. The consequences:
- **Anyone with the address can see and change everything**, other people's availability and the plan too. The preferences are only
  levels per event and task, with no free text, so there is little to leak, but they are not private.
- What makes it defensible: the app goes on an address that is not linked anywhere and is asked to stay out of
  search engines (`noindex`), and SQLite is backed up. Put it on a trusted network or behind a proxy that authenticates (see the README).
  There is no change log and no declared name (both removed 2026-11-19: "Keep it simple for now").
- Writes must be `application/json`, and `http.CrossOriginProtection` refuses writes from other sites (`Sec-Fetch-Site: cross-site`, 403; no CORS).
  All writes go through one helper, one transaction each (the one exception is deleting an organisation, above).
- Adding links or a login later is an addition and needs no new data model.

## Phases

0. **Spike (done 2026-11-03, updated 11-04)**: `solver/`, see the results below. No app yet.
1. **Backend skeleton (done 2026-11-07)**: Go, SQLite, migrations, organisations and plans, people, name picker, change log, frontend.
2. **Setup (done 2026-11-08)**: tasks, day types with places, combinations, events (singly and for all chosen weekdays in a period; self-contained, several on the same day allowed).
3. **The person page (done 2026-11-08)**: "My page" per person and plan, mobile first: choose tasks and how willingly, and answer *does not suit / poor / good* per event with one tap (tapping again removes the answer; "Set all"). Answers are saved at once. *Not built:* "its own list for this task". Narrow screens come first; a test fails if a page becomes wider than a 390 px screen.
4. **Planning**: run, preview, lock, selections, groups, explanations. **Round 1 done (2026-11-08):** "Plan" / "Try again" on the plan, background run, grid per event with explanations, compromises and distribution, check and out-of-date marking (see *Running the planner*). **Round 2 done (2026-11-09):** groups, manual cells, selections and "Lock" (see *Groups, manual cells and selections*). Optimisation happens only within the plan; no history from other plans.
5. **Grid (done 2026-11-09)**: the **grid** (events across, tasks down, fixed first column, sideways scrolling in its own box; the default on wide screens, with a "List / Grid" choice that is remembered) and **"My assignments"** at the top of My page. *No calendar feed* (dropped by the user 2026-11-09: it would show drafts without an approved plan, and events had no times of day then; times of day were added the same day, but the feed was not asked for again).
5b. **Bilingual page, run history and pin, rename and delete, README, time of day and description on events (done 2026-11-09)**: English and Norwegian page and server texts, English codes on the wire, earlier runs and a pinned current plan, rename/delete of organisations and plans, rename of people and tasks, a time of day and a description on events.
5c. **The table (done 2026-11-13)**: one view per plan for events, places per event, info rows, everyone's answers and the plan (see *The table*); replaced My page, the list and the grid.
5d. **Simpler, from the sketches (done 2026-11-19)**: the change log, the declared name and the day types removed; each event has its own places, events are copied, details
    rows can be moved; the table got the draft allocation section and dialogs for the rest (see *The table*); the setup page became the organisation page. One schema file.
6. **Deployment on kate (first test 2026-11-20)**: one image with the server and the planner (`Dockerfile`, `mise run app:image`), behind kate's Pocket ID
    forward-auth at dutyplan.haxxor.xyz (manifests in `clusters/kate/dutyplan/`). Not in a registry yet: the image is built locally and imported into the node.
    *Still to do:* a registry and automatic rollout, and a backup of the SQLite file.

**Pilot**: first two or three tasks with manual cells, groups and selections, then the rest of the tasks.

## Results from the spike (phase 0)

The solver alone, `solver/`: tests for every rule, the autumn 2026 test data, and random instances, all green.

- **The rules hold.** Every rule has a test that fails if the rule is removed (tried by breaking the model one rule at a
  time). An independent checker verifies the hard requirements on all plans. Selections, groups in manual cells (with the combination setting), proposed teams and couples, the locked
  person and the text cells work on the test data (18 people, 17 events, 7 tasks, 4 teams/couples, about 120 assignments).
- **Size and time.** After rest and the daily cap were removed, a valid and usable plan comes after about 2 seconds, and the
  quality improves up to about 20 s. The solver **does not prove** the plan is the best possible (the upper bound does not tighten), so the result
  is a *proposal* within the time limit. Different seeds give slightly different plans; "try once more" is therefore a useful button.
  Recommended time limit in the app: 30 s. Repeat with `mise run measure:time`.
- **The distribution had to be fixed twice.**
  1. Integers scaled by total capacity and `abs` gave a weak relaxation and poor convergence. Changed to a band.
  2. **Decided 2026-11-09: total load is the main term.** The spike had made the distribution *per task* the main term, because "overall first" gave a skewed distribution
     within one task (in one run one person took 7 intercessions while another got 0). The user chose total anyway: "someone who only has video should not be half
     of someone who has sound and video". With an equal share of each task, someone who takes two tasks ends up with twice as many assignments as someone who takes one (6, 12 and 6 instead of
     8, 8 and 8; tested). Measured with `mise run measure:fair` (8 s, three seeds, the autumn 2026 test data):

     | | worst deviation from own share, overall | worst deviation per task | largest spread per task |
     |---|---|---|---|
     | per task first (until 2026-11-09) | 4.3–4.5 assignments | 1.0 | 4 |
     | **total first (now)** | **0.9** | 6.6 | 10 |
     | total first + weak per task (`w_task_fair` 3) | 0.9 | 4.6–4.8 | 8 |

     **The price is that the same task can be shared unevenly among those who can do it**, since it is the total that is kept even. For a little more evenness per task without
     losing on the total, `w_task_fair` 3 (with `w_task_range` 0) is an option: on the test data the total stayed just as even.
- **History is per task** (`{person: {task: count}}`), and only works when distribution per task is on. The app does not send history.
- **The weights are not tuned against real wishes.** They are in `DEFAULT_PARAMS` and are only reasonable starting values.
- **Task pairs with no setting** count as *impractical* (the planner's `default_combination`; the app no longer lets a plan change it). In one run 9–13 people shared two
  tasks on the same day, most pairs with no setting (so impractical) and some wanted. The admin should therefore set up the
  combinations with care.

## Testing

- One small example per rule (every requirement has a test that fails if it is not met). `mise run mutations` breaks the rules in
  the planner (Python) and the app (Go, especially the organisation scoping, pinning, renaming and deleting) one at a time and requires a test to fail; quiet when everything is caught.
- Everything runs through mise (`mise.toml`: python and uv in exact versions, quiet tasks); see `CLAUDE.md`.
- An **independent checker** verifies that a finished plan breaks no hard requirement. Random instances
  are run through solver + checker.
- Texts: the Go tests keep the two language catalogues in step (same keys, same placeholders), check every key the page uses, and require a text for every error code the server can send.
  The browser check (`mise run app:ui`) runs the page in English and then in Norwegian, including the 390 px width check.
- A fixed test plan based on a church's autumn 2026 with invented names. Real names and phone and e-mail lists do not belong in the repo.

## Choices I make by default unless told otherwise

1. Name: "Dutyplan" / "Tjenesteplan" (working title). The address is decided at deployment.
2. No login in the first version. Unpublished address, `noindex`. No change log (removed 2026-11-19).
3. The solver as a Python subprocess in the same image as the Go backend.
4. One plan per period (for example a semester). Optimisation happens only within the plan; no history from earlier plans.
5. No spreadsheet support (neither import nor export).

## Risk

- The weights for points and even distribution must be tuned with real wishes, or the proposals will feel odd.
- People have to actually fill in their answers. Hence only one list per person and what is irrelevant hidden.
- Without a login a misclick or an unaware user can change other people's answers. Only the backup catches it (there is no change log
  since 2026-11-19).
- Python in the Go image makes the build and the image larger. Decided: Python is kept (see *Architecture*). Phase 6 must package Python and `ortools` into the image.
