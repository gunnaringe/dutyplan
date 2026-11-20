#!/bin/sh
# Test data for a running app: the organisation «Test» with
#   - the tasks Singing, Sound and Video, and the combinations Sound + Video = impractical (should not),
#     Sound + Singing and Video + Singing = cannot be combined,
#   - eight people (invented names) who have chosen what they can do,
#   - the plan «Autumn 2026» with an event every Sunday from 2 August to 20 December (21 of them), each needing
#     Singing 1, Sound 1 and Video 1, and the details row «Note».
# Can be run several times (what exists already is skipped; the events are added only to a plan without any). Usage: mise run app:seed [-- http://127.0.0.1:8080]
set -eu
url=${1:-http://127.0.0.1:8080}
api() { # api <method> <path> <json>
  curl -s -o /tmp/seed-body.$$ -w '%{http_code}' -X "$1" "$url$2" -H 'Content-Type: application/json' -d "$3"
}
trap 'rm -f /tmp/seed-body.$$' EXIT

code=$(api POST /api/orgs '{"name":"Test"}' || true)
case "$code" in
  201|409) ;;  # 409: exists already
  000|"") echo "seed: cannot reach $url (start the app first: mise run app:run)"; exit 1 ;;
  *) echo "seed: the organisation was refused ($code): $(cat /tmp/seed-body.$$)"; exit 1 ;;
esac
for name in Singing Sound Video; do
  code=$(api POST /api/orgs/test/tasks "{\"name\":\"$name\"}")
  case "$code" in 201|409) ;; *) echo "seed: the task $name was refused ($code): $(cat /tmp/seed-body.$$)"; exit 1 ;; esac
done

tasks=$(curl -s "$url/api/orgs/test/tasks")
id() { printf '%s' "$tasks" | sed -n "s/.*\"id\":\([0-9]*\),\"name\":\"$1\".*/\1/p"; }
sound=$(id Sound); video=$(id Video); singing=$(id Singing)
[ -n "$sound" ] && [ -n "$video" ] && [ -n "$singing" ] || { echo "seed: did not find the tasks Sound, Video and Singing in the organisation Test"; exit 1; }
combo() { # combo <task> <task> <level>
  code=$(api PUT /api/orgs/test/combinations "{\"a\":$1,\"b\":$2,\"level\":\"$3\"}")
  [ "$code" = 204 ] || { echo "seed: the combination was refused ($code): $(cat /tmp/seed-body.$$)"; exit 1; }
}
combo "$sound" "$video" impractical
combo "$sound" "$singing" cannot
combo "$video" "$singing" cannot

for name in "Kari Test" "Ola Sample" "Mona Example" "Pia Demo" "Ingrid Sample" "Njål Example" "Sunniva Test" "Jon Demo"; do
  code=$(api POST /api/orgs/test/people "{\"name\":\"$name\"}")
  case "$code" in 201|409) ;; *) echo "seed: the person $name was refused ($code): $(cat /tmp/seed-body.$$)"; exit 1 ;; esac
done

code=$(api POST /api/orgs/test/plans '{"name":"Autumn 2026"}')
case "$code" in 201|409) ;; *) echo "seed: the plan was refused ($code): $(cat /tmp/seed-body.$$)"; exit 1 ;; esac
if [ "$(curl -s "$url/api/orgs/test/plans/autumn-2026/events")" = "[]" ]; then
  # the first Sunday with one place for each task, then copies of it every week
  events=/api/orgs/test/plans/autumn-2026/events
  code=$(api POST $events '{"date":"2026-08-02"}')
  [ "$code" = 201 ] || { echo "seed: the event was refused ($code): $(cat /tmp/seed-body.$$)"; exit 1; }
  first=$(sed -n 's/^\[{"id":\([0-9]*\),.*/\1/p' /tmp/seed-body.$$)
  for task in "$singing" "$sound" "$video"; do
    code=$(api PUT "$events/$first/places/$task" '{"slots":1}')
    [ "$code" = 204 ] || { echo "seed: the places were refused ($code): $(cat /tmp/seed-body.$$)"; exit 1; }
  done
  code=$(api POST "$events/$first/copy" '{"dates":["2026-08-09"],"until":"2026-12-20","times":true,"places":true}')
  [ "$code" = 201 ] || { echo "seed: the copies were refused ($code): $(cat /tmp/seed-body.$$)"; exit 1; }
fi
code=$(api POST /api/orgs/test/plans/autumn-2026/fields '{"name":"Note"}')
case "$code" in 201|409) ;; *) echo "seed: the details row was refused ($code): $(cat /tmp/seed-body.$$)"; exit 1 ;; esac

# What people can do (answers per person): enough for «Plan» to give a plan.
people=$(curl -s "$url/api/orgs/test/people")
pid() { printf '%s' "$people" | sed -n "s/.*\"id\":\([0-9]*\),\"name\":\"$1\".*/\1/p"; }
choose() { # choose <person> <task id> <level>
  id=$(pid "$1"); [ -n "$id" ] || { echo "seed: did not find the person $1"; exit 1; }
  code=$(api PUT "/api/orgs/test/people/$id/tasks/$2" "{\"level\":\"$3\"}")
  [ "$code" = 204 ] || grep -q 'nothing to change' /tmp/seed-body.$$ || { echo "seed: the answer for $1 was refused ($code): $(cat /tmp/seed-body.$$)"; exit 1; }  # an unchanged answer is not an error
}
choose "Kari Test" "$sound" neutral;      choose "Kari Test" "$video" good
choose "Ola Sample" "$sound" good;        choose "Ola Sample" "$singing" neutral
choose "Mona Example" "$sound" neutral;  choose "Mona Example" "$singing" good
choose "Pia Demo" "$video" neutral
choose "Ingrid Sample" "$video" neutral; choose "Ingrid Sample" "$singing" good
choose "Njål Example" "$singing" neutral; choose "Njål Example" "$sound" poor
choose "Sunniva Test" "$singing" neutral;  choose "Sunniva Test" "$video" good
choose "Jon Demo" "$sound" neutral;       choose "Jon Demo" "$singing" good

echo "seed: ok. $url/o/test: Singing, Sound, Video (Sound+Video impractical, both cannot with Singing), 8 people with task choices, plan Autumn 2026 with 21 Sundays (press «Make a draft»)"
