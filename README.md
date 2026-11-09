# Dutyplan

Dutyplan (*Tjenesteplan* in Norwegian) shares out recurring duties among a group of people: who runs the sound on which
Sunday, who brings the coffee to which match, who covers which shift. People say what they can take part in and when they
cannot; a planner (a constraint solver) proposes who does what, as evenly as it can, and tells you why a place stays empty.

It is a general tool. Tasks, people, groups, events and kinds of day are all things you create yourself.

- **One small server**: a single Go binary with an embedded web page and a SQLite file. The planner is a separate Python
  program (OR-Tools CP-SAT) that the server starts when you press **Plan**.
- **English and Norwegian** (bokmål): the page follows the browser's language and has a switch in the header; the
  explanations the planner writes follow it too.
- **No login.** Read the warning below before you put it anywhere other people can reach.

## How it works

1. An **organisation** holds everything that is shared: people, tasks, kinds of day (*day types*: which tasks an event has and
   how many places each), groups (a team or a couple that works together) and which tasks can be combined on the same day.
2. An organisation has several **plans** (for example *Autumn 2026*). A plan is a list of **events**; two events may fall on
   the same day.
3. Each person has their own page (`/o/<organisation>/p/<plan>/me/<id>`; send them the link). There they pick the tasks they
   can take part in and answer *does not suit / poor / good* for each event.
4. On the plan page, **Plan** runs the planner. The result shows as a list or a grid, with the reasons for anything left
   empty. A person can be **locked** onto a place (a *manual cell*), and a *selection* limits who can be picked for a task on
   an event. **Weighting** says what the planner cares about; the default is an equal *total* load per person.
5. Each run is kept. **Pin** a finished run as the plan's current plan and everybody's *My assignments* follows it; with
   nothing pinned they see the newest finished run.

Everything you create can be renamed, and plans and organisations can be deleted (an organisation only when it has no plans).
Every change is recorded in a change log with the name the person typed in under **Who are you?**.

## No login: where to run it

Anyone who can reach the address can read and change **everything**. The name in *Who are you?* is self-declared and is only
used in the change log; it is not authentication. So:

- run it on a network you trust (a home or office LAN), or
- put it behind a reverse proxy that does the authentication (HTTP basic auth, an identity-aware proxy, a VPN), and do not
  expose the port itself to the internet.

The server sends `X-Robots-Tag: noindex` and a strict Content-Security-Policy, but that only keeps it out of search engines.

## Requirements

| What | For | Version |
| --- | --- | --- |
| Go | building the server | 1.26 |
| Python with [OR-Tools](https://developers.google.com/optimization) | the planner (optional: without it everything works except **Plan**) | 3.12 (newer ones that OR-Tools has wheels for also work) |
| [mise](https://mise.jdx.dev) | the easy way to get both, pinned (see `mise.toml`) | recent |

The database is SQLite through a pure-Go driver, so there is nothing to install for it.

## Quick start

With mise:

```sh
mise install            # Go, Python and uv, in the versions pinned in mise.toml
mise run setup          # the Python packages (OR-Tools) for the planner
mise run app:run        # starts on http://127.0.0.1:8080 and prints the address
```

`mise.toml` sets `DUTYPLAN_SOLVER` so that the server finds the planner. Open the address, create an organisation, add tasks
and a day type under **Setup**, create a plan, add events and people, let people answer, and press **Plan**.
`mise run app:seed -- http://127.0.0.1:8080` fills a running server with a small demo organisation.

Without mise:

```sh
go build -o dutyplan ./cmd/dutyplan

python3.12 -m venv planner-venv           # a Python that OR-Tools has wheels for
planner-venv/bin/pip install ./solver     # the planner and OR-Tools

DUTYPLAN_SOLVER="$PWD/planner-venv/bin/python -m dutyplan_solver" ./dutyplan --db /var/lib/dutyplan/dutyplan.db
```

## Options

| Flag | Default | |
| --- | --- | --- |
| `--addr` | `127.0.0.1:8080` | where to listen; port `0` picks a free one. Use `0.0.0.0:8080` to be reachable from other machines (see the warning above). |
| `--db` | `dutyplan.db` | the SQLite file; created if missing. Its directory must be writable (SQLite keeps `-wal` and `-shm` files next to it). |
| `--solver` | `$DUTYPLAN_SOLVER` | the command that starts the planner, split on spaces (no quoting). Empty means no planning. |
| `--workers` | CPUs, at most 8 | threads the planner may use. |

`DUTYPLAN_SOLVER` is the same as `--solver`. The server writes one line with its address to standard output and logs to
standard error. It stops cleanly on `SIGINT` and `SIGTERM`. A planner run that was going when the server stopped is marked as
failed at the next start.

Only one planner run goes at a time. A run has a time limit (10, 20 or 60 seconds on the page); it stops when it has found
the best plan or runs out of time, and says which.

## Data and backups

All data is one SQLite file (plus the `-wal`/`-shm` files while the server runs). The schema is upgraded automatically at
startup by forward-only migrations, so **take a backup before upgrading**. A consistent copy of a running server:

```sh
sqlite3 dutyplan.db ".backup 'dutyplan-backup.db'"
```

or stop the server and copy the three files. The 20 newest runs of each plan are kept, plus the pinned one; older runs are
removed when a new one starts.

Deleting a plan removes its events, the answers people gave for them, manual cells, selections and runs, and keeps the change
log. Deleting an organisation removes everything in it, its change log included, so that one deletion is written to the
server's log (standard error) instead.

## Development

Everything goes through mise; `mise tasks` lists the tasks.

| Task | |
| --- | --- |
| `mise run test` | all tests, Go and Python, quiet (about 15 s) |
| `mise run test:fast` | the quickest loop: Go and the planner's rule tests |
| `mise run mutations` | breaks every rule in turn and checks that a test fails (about 7 min) |
| `mise run app:smoke` | starts the server on a temporary database and checks the tenant isolation with curl |
| `mise run app:ui` | browser check of the page in both languages (needs `playwright-core` and Chromium; see the script) |
| `mise run verify` | tests, mutations and the smoke test: everything that must be green before a commit |

Layout: `cmd/dutyplan` (main), `internal/store` (SQLite, migrations, all data access), `internal/planner` (builds the planner's
input from the database, runs it, turns the answer into a view), `internal/api` (JSON API), `internal/i18n` (server texts),
`web/` (the page: plain JavaScript, no build step), `solver/` (the Python planner and its tests). `CLAUDE.md` holds the design
rules and conventions for anyone (or any assistant) working on the code; `PLAN.md` has the design notes.

## License

Apache License 2.0, see `LICENSE`.
