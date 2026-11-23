#!/bin/sh
# Smoke test of the built binary over its Connect API: two organisations, isolation, cross-site writes, a real planning run, pin, rename, delete, restart.
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

# call <expected> <Service/Method> [json] [header]: a call of the API (Connect: POST /dutyplan.v1.<Service>/<Method> with
# JSON); checks the status code and leaves the answer in $tmp/body. code <expected> <path>: a GET of a page.
call() {
  want=$1; proc=$2; body=${3:-"{}"}; header=${4:-}
  set -- -s -o "$tmp/body" -w '%{http_code}' -X POST "$url/dutyplan.v1.$proc" -H 'Content-Type: application/json' -d "$body"
  [ -z "$header" ] || set -- "$@" -H "$header"
  got=$(curl "$@")
  [ "$got" = "$want" ] || fail "$proc $body gave $got, expected $want ($(cat "$tmp/body"))"
}
code() {
  got=$(curl -s -o "$tmp/body" -w '%{http_code}' "$url$2")
  [ "$got" = "$1" ] || fail "GET $2 gave $got, expected $1"
}
A='"org":"org-a"'; B='"org":"org-b"'; T='"org":"test"'

start
call 200 AccountService/CreateOrg '{"name":"Org A"}'
call 200 AccountService/CreateOrg '{"name":"Org B"}'
call 200 PlanService/CreatePlan "{$A,\"name\":\"Autumn 2026\"}"
call 200 PlanService/CreatePlan "{$B,\"name\":\"Autumn 2026\"}"
call 200 SetupService/CreatePerson "{$A,\"name\":\"Kari Test\"}"
id=$(sed -n 's/.*"id":\([0-9]*\).*/\1/p' "$tmp/body")
[ -n "$id" ] || fail "did not find the person id"

call 200 PlanService/CreatePlan "{$A,\"name\":\"Spring 2027\"}"          # exists only in A

call 403 AccountService/CreateOrg '{"name":"Cross site"}' 'Sec-Fetch-Site: cross-site'   # a call from another site
call 404 SetupService/UpdatePerson "{$B,\"id\":$id,\"proposable\":false}"    # A's person through B
call 404 PlanService/GetPlan "{$B,\"plan\":\"spring-2027\"}"                  # A's plan through B
call 200 PlanService/GetPlan "{$A,\"plan\":\"spring-2027\"}"
call 200 PlanService/ListPlans "{$B}"
[ "$(grep -o '"slug"' "$tmp/body" | wc -l)" -eq 1 ] || fail "B sees plans other than its own: $(cat "$tmp/body")"
call 200 SetupService/ListPeople "{$A}"
grep -q '"proposable":true' "$tmp/body" || fail "A's person was changed through B"
code 200 / && grep -q Dutyplan "$tmp/body" || fail "the front page is missing"
code 200 /i18n.js && grep -q MESSAGES "$tmp/body" || fail "the texts of the page are missing"
code 200 /o/org-a/p/autumn-2026

call 200 SetupService/CreateTask "{$A,\"name\":\"Doors\"}"
task=$(sed -n 's/.*"id":\([0-9]*\).*/\1/p' "$tmp/body")
[ -n "$task" ] || fail "did not find the task id"
call 200 SetupService/UpdateTask "{$A,\"id\":$task,\"planned\":false}"   # filled by hand only, so org-a has nothing to plan
call 200 PlanService/AddEvents "{$B,\"plan\":\"autumn-2026\",\"date\":\"2026-08-09\"}"
evb=$(sed -n 's/^{"events":\[{"id":\([0-9]*\),.*/\1/p' "$tmp/body")
call 404 PlanService/SetPlaces "{$B,\"plan\":\"autumn-2026\",\"event\":$evb,\"task\":$task,\"slots\":1}"   # A's task on B's event
call 200 PlanService/DeleteEvent "{$B,\"plan\":\"autumn-2026\",\"id\":$evb}"
call 200 PlanService/AddEvents "{$A,\"plan\":\"autumn-2026\",\"date\":\"2026-08-09\"}"
ev=$(sed -n 's/^{"events":\[{"id":\([0-9]*\),.*/\1/p' "$tmp/body")
call 200 PlanService/SetPlaces "{$A,\"plan\":\"autumn-2026\",\"event\":$ev,\"task\":$task,\"slots\":2}"
call 200 PlanService/AddEvents "{$A,\"plan\":\"autumn-2026\",\"date\":\"2026-08-09\"}"   # two events on the same day
call 200 PlanService/AddEvents "{$A,\"plan\":\"autumn-2026\",\"date\":\"2026-08-09\",\"start_time\":\"11:00\",\"end_time\":\"12:30\"}"
timed=$(sed -n 's/^{"events":\[{"id":\([0-9]*\),.*/\1/p' "$tmp/body")
call 200 PlanService/UpdateEvent "{$A,\"plan\":\"autumn-2026\",\"id\":$timed,\"end_time\":\"13:00\"}"
grep -q '"start_time":"11:00"' "$tmp/body" && grep -q '"end_time":"13:00"' "$tmp/body" || fail "the times did not come back: $(cat "$tmp/body")"
call 400 PlanService/AddEvents "{$A,\"plan\":\"autumn-2026\",\"date\":\"2026-08-09\",\"start_time\":\"25:00\"}"
grep -q '"code":"start_time_invalid"' "$tmp/body" || fail "a bad time gave the wrong answer: $(cat "$tmp/body")"
call 200 PlanService/ListEvents "{$A,\"plan\":\"autumn-2026\"}"
[ "$(grep -o '"start_time":"[^"]*"' "$tmp/body" | head -1)" = '"start_time":"11:00"' ] || fail "the event with a time is not first on its date: $(cat "$tmp/body")"
call 200 PlanService/ListEvents "{$B,\"plan\":\"autumn-2026\"}"
[ "$(cat "$tmp/body")" = '{"events":[]}' ] || fail "B sees A's dates: $(cat "$tmp/body")"

sh scripts/seed-demo.sh "$url" >/dev/null || fail "the seed script failed"
sh scripts/seed-demo.sh "$url" >/dev/null || fail "the seed script failed on the second run"   # idempotent: nothing is doubled
call 200 SetupService/ListPeople "{$T}"
[ "$(grep -o '"name"' "$tmp/body" | wc -l)" -eq 8 ] || fail "the seed should give 8 people: $(cat "$tmp/body")"
call 200 PlanService/ListEvents "{$T,\"plan\":\"autumn-2026\"}"
[ "$(grep -o '"date"' "$tmp/body" | wc -l)" -eq 21 ] || fail "the seed should give 21 events (Sundays 2 Aug - 20 Dec)"
call 200 SetupService/ListCombinations "{$T}"
[ "$(grep -o '"level":"cannot"' "$tmp/body" | wc -l)" -eq 2 ] && [ "$(grep -o '"level"' "$tmp/body" | wc -l)" -eq 2 ] || fail "the seed's combinations are wrong: $(cat "$tmp/body")"

# planning with the real planner: the run is done, checked without violations, and invisible to another organisation
# (org-a has a plan with the same address, autumn-2026)
[ -n "${DUTYPLAN_SOLVER:-}" ] || fail "DUTYPLAN_SOLVER is not set (run via mise run app:smoke)"
TP="$T,\"plan\":\"autumn-2026\""; AP="$A,\"plan\":\"autumn-2026\""
call 200 DraftService/StartRun "{$TP,\"time_limit\":5}"
run=$(sed -n 's/.*"id":\([0-9]*\).*/\1/p' "$tmp/body")
[ -n "$run" ] || fail "did not find the run id"
status=running; i=0
while [ "$status" = running ] && [ "$i" -lt 150 ]; do
  call 200 DraftService/GetRun "{$TP,\"id\":$run}"
  status=$(sed -n 's/.*"status":"\([a-z]*\)".*/\1/p' "$tmp/body")
  [ "$status" != running ] || { i=$((i + 1)); sleep 0.2; }
done
[ "$status" = done ] || fail "planning did not finish ($status): $(head -c 400 "$tmp/body")"
grep -q '"checked":true' "$tmp/body" && grep -q '"violations":\[\]' "$tmp/body" || fail "the check found violations or did not run: $(head -c 400 "$tmp/body")"
[ "$(grep -o '"cells"' "$tmp/body" | wc -l)" -eq 21 ] || fail "the plan should have a row per event (21)"
call 404 DraftService/GetRun "{$AP,\"id\":$run}"
call 404 DraftService/GetRun "{$AP}"                                    # the newest: org-a has none
call 400 DraftService/StartRun "{$AP,\"time_limit\":5}"               # org-a has no tasks to distribute

# pin: another organisation cannot touch the run; the pin shows in the list; it can be taken back
call 404 DraftService/PinRun "{$AP,\"id\":$run}"
call 200 DraftService/PinRun "{$TP,\"id\":$run}"
call 200 DraftService/ListRuns "{$TP}"
grep -q '"pinned":true' "$tmp/body" || fail "the pinned run is not marked in the list: $(cat "$tmp/body")"

# rename keeps the address; deleting needs the right organisation
call 200 PlanService/RenamePlan "{$TP,\"name\":\"Fall 2026\"}"
call 200 PlanService/GetPlan "{$TP}"
grep -q '"name":"Fall 2026"' "$tmp/body" || fail "the rename did not stick"
call 404 PlanService/DeletePlan "{$B,\"plan\":\"spring-2027\"}"       # exists only in A

stop
start                                                                  # the same database after a restart
call 200 AccountService/ListOrgs
grep -q '"org-a"' "$tmp/body" && grep -q '"org-b"' "$tmp/body" || fail "the organisations are gone after a restart"
call 200 PlanService/ListPlaces "{$AP}"
grep -q "\"task_id\":$task,\"slots\":2" "$tmp/body" || fail "the places are gone after a restart: $(cat "$tmp/body")"
call 200 DraftService/GetRun "{$TP}"
grep -q '"status":"done"' "$tmp/body" && grep -q '"pinned":true' "$tmp/body" || fail "the plan and its pin are gone after a restart"

# deleting: the plan with everything in it, then the organisation (which needs its address as confirmation)
call 200 PlanService/DeletePlan "{$TP}"
call 404 DraftService/GetRun "{$TP}"
call 400 OrgService/DeleteOrg "{$T}"
call 200 OrgService/DeleteOrg "{$T,\"confirm\":\"test\"}"
call 404 OrgService/GetOrg "{$T}"
call 200 PlanService/GetPlan "{$AP}"                                    # the other organisations are untouched
echo "smoke: ok"
