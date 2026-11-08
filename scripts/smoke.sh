#!/bin/sh
# Røyktest av den bygde binæren: to organisasjoner, isolasjon, aktørkrav, restart. Én linje ved suksess.
# Forutsetter bin/tjenesteplan (mise run app:build). Stopper serveren på PID-en den startet selv.
set -eu
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
pid=""
cleanup() {
  [ -z "$pid" ] || { kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; }
  rm -rf "$tmp"
}
trap cleanup EXIT
fail() { echo "smoke: FEIL: $*"; [ ! -s "$tmp/err" ] || sed 's/^/  server: /' "$tmp/err" | tail -5; exit 1; }

start() {
  : >"$tmp/out"
  bin/tjenesteplan --addr 127.0.0.1:0 --db "$tmp/smoke.db" >"$tmp/out" 2>"$tmp/err" &
  pid=$!
  url=""
  i=0
  while [ "$i" -lt 50 ]; do
    url=$(sed -n 's/^tjenesteplan: //p' "$tmp/out")
    [ -z "$url" ] || break
    i=$((i + 1)); sleep 0.1
  done
  [ -n "$url" ] || fail "serveren startet ikke"
}
stop() { kill "$pid"; wait "$pid" 2>/dev/null || true; pid=""; }

# code <forventet> <metode> <sti> [json] [actor]: sjekker statuskoden; svaret ligger i $tmp/body
code() {
  want=$1; method=$2; path=$3; body=${4:-}; actor=${5-Smoke}
  set -- -s -o "$tmp/body" -w '%{http_code}' -X "$method" "$url$path"
  [ -z "$body" ] || set -- "$@" -H 'Content-Type: application/json' -d "$body"
  [ -z "$actor" ] || set -- "$@" -H "X-Actor: $actor"
  got=$(curl "$@")
  [ "$got" = "$want" ] || fail "$method $path ga $got, ventet $want ($(cat "$tmp/body"))"
}

start
code 201 POST /api/orgs '{"name":"Org A"}'
code 201 POST /api/orgs '{"name":"Org B"}'
code 201 POST /api/orgs/org-a/plans '{"name":"Høst 2026"}'
code 201 POST /api/orgs/org-b/plans '{"name":"Høst 2026"}'
code 201 POST /api/orgs/org-a/people '{"name":"Kari Test"}'
id=$(sed -n 's/.*"id":\([0-9]*\).*/\1/p' "$tmp/body")
[ -n "$id" ] || fail "fant ikke person-id"

code 201 POST /api/orgs/org-a/plans '{"name":"Vår 2027"}'            # finnes bare i A

code 400 POST /api/orgs '{"name":"Uten aktør"}' ""                    # ingen X-Actor
code 404 PATCH "/api/orgs/org-b/people/$id" '{"proposable":false}'    # A sin person via B
code 404 GET /api/orgs/org-b/plans/var-2027                           # A sin plan via B
code 200 GET /api/orgs/org-a/plans/var-2027
code 200 GET /api/orgs/org-b/plans
[ "$(grep -o '"slug"' "$tmp/body" | wc -l)" -eq 1 ] || fail "B ser andre planer enn sine egne: $(cat "$tmp/body")"
code 200 GET /api/orgs/org-a/people
grep -q '"proposable":true' "$tmp/body" || fail "A sin person ble endret via B"
code 200 GET / && grep -q Tjenesteplan "$tmp/body" || fail "forsiden mangler"
code 200 GET /o/org-a/p/host-2026

code 201 POST /api/orgs/org-a/day-types '{"name":"Helg"}'
dt=$(sed -n 's/.*"id":\([0-9]*\).*/\1/p' "$tmp/body")
[ -n "$dt" ] || fail "fant ikke dagtype-id"
code 404 POST /api/orgs/org-b/plans/host-2026/events "{\"date\":\"2026-08-09\",\"day_type_id\":$dt}"   # A sin dagtype på B sin dato
code 201 POST /api/orgs/org-a/plans/host-2026/events "{\"date\":\"2026-08-09\",\"day_type_id\":$dt}"
code 201 POST /api/orgs/org-a/plans/host-2026/events "{\"date\":\"2026-08-09\",\"day_type_id\":$dt}"   # to hendelser samme dag
code 200 GET /api/orgs/org-b/plans/host-2026/events
[ "$(cat "$tmp/body")" = "[]" ] || fail "B ser A sine datoer: $(cat "$tmp/body")"

stop
start                                                                  # samme database etter omstart
code 200 GET /api/orgs
grep -q '"org-a"' "$tmp/body" && grep -q '"org-b"' "$tmp/body" || fail "organisasjonene ligger ikke igjen etter omstart"
code 200 GET /api/orgs/org-a/log
[ "$(grep -o '"action"' "$tmp/body" | wc -l)" -ge 4 ] || fail "endringsloggen ligger ikke igjen etter omstart"
echo "smoke: ok"
