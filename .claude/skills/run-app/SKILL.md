---
name: run-app
description: Start and try the Dutyplan app (a Go server with an embedded frontend) locally, or see that it works without reading code. Use when someone wants to see the app, call the API, or check organisations and plans.
---

Everything through mise. No login; the organisation is in the address.

- **See that everything works without opening anything:** `mise run app:smoke` (about 2 s; starts the binary on a temporary database, two organisations,
  isolation, the actor requirement, a real planning run, a restart; stops it again; one line) and `mise run app:ui` (headless browser, about 20 s, English and then Norwegian).
- **Run it yourself:** `mise run app:run -- --addr 127.0.0.1:8080 --db /tmp/dp.db` in the background (prints `dutyplan: http://...`). Stop it
  on the PID you started, not with `pkill`. The default database `dutyplan.db` is in the working directory; use `--db /tmp/...` for trying things.
- **Test data:** `mise run app:seed -- <address>` puts the organisation «Test» (`/o/test`) with the tasks Singing, Sound and Video into a running app, with Sound+Video impractical and both «cannot» with Singing, the day type Service, eight people who have chosen tasks, and the plan Autumn 2026 with 21 Sundays; afterwards «Plan» gives a plan. Safe to run several times.
- **API (JSON):** `GET /api/orgs`, `POST /api/orgs {"name"}`, `PATCH|DELETE /api/orgs/{org}` (rename; delete needs `?confirm=<slug>` and no plans), `GET|POST /api/orgs/{org}/plans`, `PATCH|DELETE /plans/{plan}`, `/people`, `PATCH /people/{id}`,
  `/tasks`, `/day-types` (+ `PUT /{id}/tasks`), `/combinations`, and per plan `/plans/{plan}/events` (+ `/bulk`, `PATCH|DELETE /{id}`; an event has `date`, `day_type_id`, `note`, `start_time`/`end_time` as `HH:MM` or `""`, and `description`).
  Groups: `GET|POST /groups`, `PATCH|DELETE /groups/{id}`, `PUT /groups/{id}/members` (`{people:[id]}`) and `/tasks` (`{tasks:[{task_id,fills,level}]}`). A person's assignments in the current plan: `GET /plans/{plan}/people/{id}/assignments`. Weighting per plan: `GET|PUT /plans/{plan}/weights` (`{values:{w_fair: 12, "w_task.good": 3, default_combination: "practical"}}`; replaces everything, `{}` is the default; the answer lists all settings with default and limits). Per plan: manual cells `GET|POST /plans/{plan}/cells` (`{events:[id], task, person|group|label, fills}`), `DELETE /cells/{id}`, selections `GET|PUT /plans/{plan}/pools` (`{events:[id], task, people:[id], groups:[id]}`; empty removes it).
  `GET /api/orgs/{org}/log?plan=&limit=`. A person's answers: `GET /plans/{plan}/people/{id}/me`, `PUT|DELETE /people/{id}/tasks/{task}` (`{level}`), `PUT /plans/{plan}/people/{id}/availability` (`{levels: {eventId: never|poor|good|""}}`). The table (per plan): places per event `GET /plans/{plan}/places`, `PUT|DELETE /plans/{plan}/events/{id}/places/{task}` (`{slots}`; DELETE goes back to the day type's), info rows `GET|POST /plans/{plan}/fields`, `PATCH|DELETE /fields/{id}`, `PUT /plans/{plan}/events/{id}/fields/{field}` (`{value}`), everyone's answers `GET /plans/{plan}/answers`, and one task against all others `PUT /orgs/{org}/tasks/{id}/combinations` (`{level}`).
  Writes need `Content-Type: application/json` and `X-Actor: <percent-encoded name>`.
  Texts that explain a run are written in `?lang=en|nb` (English by default). Errors are `{error, code, params, message}`.
  Example: `curl -s -H 'Content-Type: application/json' -H 'X-Actor: Ola' -d '{"name":"Sports Club"}' $URL/api/orgs`.
- **Planning:** `mise run app:run` has the planner through `$DUTYPLAN_SOLVER`; without it `POST /api/orgs/{org}/plans/{plan}/runs` answers 503. Start: `POST` (`{"time_limit":10}`, optional) gives `{id}` at once; fetch `GET .../runs/{id|latest}` until `run.status` is `done` or `failed` (list: `GET .../runs`). `run.violations` should be empty; `run.stale` means answers or setup have changed since. Pin a finished run as the current plan with `PUT .../runs/{id}/pin` (`DELETE` unpins); the page opens with the pinned run (else the newest), and `/assignments` follows the pinned run, else the newest finished one.
- Use only invented names in the data you enter and commit.
