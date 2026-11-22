#!/bin/sh
# Smoke test of the built binary: two organisations, isolation, cross-site writes, a real planning run, pin, rename, delete, restart.
# One line on success. Needs bin/dutyplan (mise run app:build). Stops the server on the PID it started itself.
set -eu
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
pid=""
cleanup() {
  [ -z "$pid" ] || { kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; }
  rm -rf "$tmp"
}
trap cleanup EXIT
fail() { echo "smoke: FAILED: $*"; [ ! -s "$tmp/err" ] || sed 's/^/  server: /' "$tmp/err" | tail -5; exit 1; }

start() {
  : >"$tmp/out"
  bin/dutyplan --addr 127.0.0.1:0 --db "$tmp/smoke.db" >"$tmp/out" 2>"$tmp/err" &
  pid=$!
  url=""
  i=0
  while [ "$i" -lt 50 ]; do
    url=$(sed -n 's/^dutyplan: //p' "$tmp/out")
    [ -z "$url" ] || break
    i=$((i + 1)); sleep 0.1
  done
  [ -n "$url" ] || fail "the server did not start"
}
stop() { kill "$pid"; wait "$pid" 2>/dev/null || true; pid=""; }

# code <expected> <method> <path> [json] [header]: checks the status code; the answer is left in $tmp/body
code() {
  want=$1; method=$2; path=$3; body=${4:-}; header=${5:-}
  set -- -s -o "$tmp/body" -w '%{http_code}' -X "$method" "$url$path"
  [ -z "$body" ] || set -- "$@" -H 'Content-Type: application/json' -d "$body"
  [ -z "$header" ] || set -- "$@" -H "$header"
  got=$(curl "$@")
  [ "$got" = "$want" ] || fail "$method $path gave $got, expected $want ($(cat "$tmp/body"))"
}

start
code 201 POST /api/orgs '{"name":"Org A"}'
code 201 POST /api/orgs '{"name":"Org B"}'
code 201 POST /api/orgs/org-a/plans '{"name":"Autumn 2026"}'
code 201 POST /api/orgs/org-b/plans '{"name":"Autumn 2026"}'
code 201 POST /api/orgs/org-a/people '{"name":"Kari Test"}'
id=$(sed -n 's/.*"id":\([0-9]*\).*/\1/p' "$tmp/body")
[ -n "$id" ] || fail "did not find the person id"

code 201 POST /api/orgs/org-a/plans '{"name":"Spring 2027"}'          # exists only in A

code 403 POST /api/orgs '{"name":"Cross site"}' 'Sec-Fetch-Site: cross-site'   # a write from another site
code 404 PATCH "/api/orgs/org-b/people/$id" '{"proposable":false}'    # A's person through B
code 404 GET /api/orgs/org-b/plans/spring-2027                        # A's plan through B
code 200 GET /api/orgs/org-a/plans/spring-2027
code 200 GET /api/orgs/org-b/plans
[ "$(grep -o '"slug"' "$tmp/body" | wc -l)" -eq 1 ] || fail "B sees plans other than its own: $(cat "$tmp/body")"
code 200 GET /api/orgs/org-a/people
grep -q '"proposable":true' "$tmp/body" || fail "A's person was changed through B"
code 200 GET / && grep -q Dutyplan "$tmp/body" || fail "the front page is missing"
code 200 GET /i18n.js && grep -q MESSAGES "$tmp/body" || fail "the texts of the page are missing"
code 200 GET /o/org-a/p/autumn-2026

code 201 POST /api/orgs/org-a/tasks '{"name":"Doors"}'
task=$(sed -n 's/.*"id":\([0-9]*\).*/\1/p' "$tmp/body")
[ -n "$task" ] || fail "did not find the task id"
code 200 PATCH "/api/orgs/org-a/tasks/$task" '{"planned":false}'   # filled by hand only, so org-a has nothing to plan
code 201 POST /api/orgs/org-b/plans/autumn-2026/events '{"date":"2026-08-09"}'
evb=$(sed -n 's/^\[{"id":\([0-9]*\),.*/\1/p' "$tmp/body")
code 404 PUT "/api/orgs/org-b/plans/autumn-2026/events/$evb/places/$task" '{"slots":1}'   # A's task on B's event
code 204 DELETE "/api/orgs/org-b/plans/autumn-2026/events/$evb"
code 201 POST /api/orgs/org-a/plans/autumn-2026/events '{"date":"2026-08-09"}'
ev=$(sed -n 's/^\[{"id":\([0-9]*\),.*/\1/p' "$tmp/body")
code 204 PUT "/api/orgs/org-a/plans/autumn-2026/events/$ev/places/$task" '{"slots":2}'
code 201 POST /api/orgs/org-a/plans/autumn-2026/events '{"date":"2026-08-09"}'   # two events on the same day
code 201 POST /api/orgs/org-a/plans/autumn-2026/events '{"date":"2026-08-09","start_time":"11:00","end_time":"12:30"}'
timed=$(sed -n 's/^\[{"id":\([0-9]*\),.*/\1/p' "$tmp/body")
code 200 PATCH "/api/orgs/org-a/plans/autumn-2026/events/$timed" '{"end_time":"13:00"}'
grep -q '"start_time":"11:00"' "$tmp/body" && grep -q '"end_time":"13:00"' "$tmp/body" || fail "the times did not come back: $(cat "$tmp/body")"
code 400 POST /api/orgs/org-a/plans/autumn-2026/events '{"date":"2026-08-09","start_time":"25:00"}'
grep -q '"code":"start_time_invalid"' "$tmp/body" || fail "a bad time gave the wrong answer: $(cat "$tmp/body")"
code 200 GET /api/orgs/org-a/plans/autumn-2026/events
[ "$(grep -o '"start_time":"[^"]*"' "$tmp/body" | head -1)" = '"start_time":"11:00"' ] || fail "the event with a time is not first on its date: $(cat "$tmp/body")"
code 200 GET /api/orgs/org-b/plans/autumn-2026/events
[ "$(cat "$tmp/body")" = "[]" ] || fail "B sees A's dates: $(cat "$tmp/body")"

sh scripts/seed-demo.sh "$url" >/dev/null || fail "the seed script failed"
sh scripts/seed-demo.sh "$url" >/dev/null || fail "the seed script failed on the second run"   # idempotent: nothing is doubled
code 200 GET /api/orgs/test/people
[ "$(grep -o '"name"' "$tmp/body" | wc -l)" -eq 8 ] || fail "the seed should give 8 people: $(cat "$tmp/body")"
code 200 GET /api/orgs/test/plans/autumn-2026/events
[ "$(grep -o '"date"' "$tmp/body" | wc -l)" -eq 21 ] || fail "the seed should give 21 events (Sundays 2 Aug - 20 Dec)"
code 200 GET /api/orgs/test/combinations
[ "$(grep -o '"level":"cannot"' "$tmp/body" | wc -l)" -eq 2 ] && [ "$(grep -o '"level"' "$tmp/body" | wc -l)" -eq 2 ] || fail "the seed's combinations are wrong: $(cat "$tmp/body")"

# planning with the real planner: the run is done, checked without violations, and invisible to another organisation
# (org-a has a plan with the same address, autumn-2026)
[ -n "${DUTYPLAN_SOLVER:-}" ] || fail "DUTYPLAN_SOLVER is not set (run via mise run app:smoke)"
code 202 POST /api/orgs/test/plans/autumn-2026/runs '{"time_limit":5}'
run=$(sed -n 's/.*"id":\([0-9]*\).*/\1/p' "$tmp/body")
[ -n "$run" ] || fail "did not find the run id"
status=running; i=0
while [ "$status" = running ] && [ "$i" -lt 150 ]; do
  code 200 GET "/api/orgs/test/plans/autumn-2026/runs/$run"
  status=$(sed -n 's/.*"status":"\([a-z]*\)".*/\1/p' "$tmp/body")
  [ "$status" != running ] || { i=$((i + 1)); sleep 0.2; }
done
[ "$status" = done ] || fail "planning did not finish ($status): $(head -c 400 "$tmp/body")"
grep -q '"checked":true' "$tmp/body" && grep -q '"violations":\[\]' "$tmp/body" || fail "the check found violations or did not run: $(head -c 400 "$tmp/body")"
[ "$(grep -o '"cells"' "$tmp/body" | wc -l)" -eq 21 ] || fail "the plan should have a row per event (21)"
code 404 GET "/api/orgs/org-a/plans/autumn-2026/runs/$run"
code 404 GET /api/orgs/org-a/plans/autumn-2026/runs/latest
code 400 POST /api/orgs/org-a/plans/autumn-2026/runs '{"time_limit":5}'        # org-a has no tasks to distribute

# pin: another organisation cannot touch the run; the pin shows in the list; it can be taken back
code 404 PUT "/api/orgs/org-a/plans/autumn-2026/runs/$run/pin"
code 204 PUT "/api/orgs/test/plans/autumn-2026/runs/$run/pin"
code 200 GET /api/orgs/test/plans/autumn-2026/runs
grep -q '"pinned":true' "$tmp/body" || fail "the pinned run is not marked in the list: $(cat "$tmp/body")"

# rename keeps the address; deleting needs the right organisation
code 200 PATCH /api/orgs/test/plans/autumn-2026 '{"name":"Fall 2026"}'
code 200 GET /api/orgs/test/plans/autumn-2026
grep -q '"name":"Fall 2026"' "$tmp/body" || fail "the rename did not stick"
code 404 DELETE /api/orgs/org-b/plans/spring-2027                      # exists only in A

stop
start                                                                  # the same database after a restart
code 200 GET /api/orgs
grep -q '"org-a"' "$tmp/body" && grep -q '"org-b"' "$tmp/body" || fail "the organisations are gone after a restart"
code 200 GET /api/orgs/org-a/plans/autumn-2026/places
grep -q "\"task_id\":$task,\"slots\":2" "$tmp/body" || fail "the places are gone after a restart: $(cat "$tmp/body")"
code 200 GET "/api/orgs/test/plans/autumn-2026/runs/latest"
grep -q '"status":"done"' "$tmp/body" && grep -q '"pinned":true' "$tmp/body" || fail "the plan and its pin are gone after a restart"

# deleting: the plan with everything in it, then the organisation (which needs its address as confirmation)
code 204 DELETE /api/orgs/test/plans/autumn-2026
code 404 GET /api/orgs/test/plans/autumn-2026/runs/latest
code 400 DELETE /api/orgs/test
code 204 DELETE "/api/orgs/test?confirm=test"
code 404 GET /api/orgs/test
code 200 GET /api/orgs/org-a/plans/autumn-2026                          # the other organisations are untouched
echo "smoke: ok"
