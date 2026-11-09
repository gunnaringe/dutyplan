#!/bin/sh
# Testdata til en kjørende app: organisasjonen «Test» med
#   - oppgavene Lovsang, Lyd og Bilde, og kombinasjonene Lyd + Bilde = impractical (bør ikke),
#     Lyd + Lovsang og Bilde + Lovsang = kan ikke kombineres,
#   - åtte personer (oppdiktede navn) som har valgt hva de kan stille på,
#   - dagtypen «Gudstjeneste» (Lovsang 1, Lyd 1, Bilde 1 plass),
#   - planen «Høst 2026» med en hendelse hver søndag 2. august - 20. desember (21 stk).
# Kan kjøres flere ganger (det som finnes fra før, hoppes over). Bruk: mise run app:seed [-- http://127.0.0.1:8080]
set -eu
url=${1:-http://127.0.0.1:8080}
api() { # api <metode> <sti> <json>
  curl -s -o /tmp/seed-body.$$ -w '%{http_code}' -X "$1" "$url$2" -H 'Content-Type: application/json' -H 'X-Actor: Seed' -d "$3"
}
trap 'rm -f /tmp/seed-body.$$' EXIT

code=$(api POST /api/orgs '{"name":"Test","slug":"test"}' || true)
case "$code" in
  201|409) ;;  # 409: finnes fra før
  000|"") echo "seed: får ikke kontakt med $url (start appen først: mise run app:run)"; exit 1 ;;
  *) echo "seed: organisasjonen ble avvist ($code): $(cat /tmp/seed-body.$$)"; exit 1 ;;
esac
code=$(api POST /api/orgs/test/tasks/bulk '{"names":"Lovsang\nLyd\nBilde"}')
[ "$code" = 201 ] || { echo "seed: oppgavene ble avvist ($code): $(cat /tmp/seed-body.$$)"; exit 1; }

tasks=$(curl -s "$url/api/orgs/test/tasks")
id() { printf '%s' "$tasks" | sed -n "s/.*\"id\":\([0-9]*\),\"name\":\"$1\".*/\1/p"; }
lyd=$(id Lyd); bilde=$(id Bilde); lovsang=$(id Lovsang)
[ -n "$lyd" ] && [ -n "$bilde" ] && [ -n "$lovsang" ] || { echo "seed: fant ikke oppgavene Lyd, Bilde og Lovsang i organisasjonen Test"; exit 1; }
combo() { # combo <oppgave> <oppgave> <nivå>
  code=$(api PUT /api/orgs/test/combinations "{\"a\":$1,\"b\":$2,\"level\":\"$3\"}")
  [ "$code" = 204 ] || { echo "seed: kombinasjonen ble avvist ($code): $(cat /tmp/seed-body.$$)"; exit 1; }
}
combo "$lyd" "$bilde" impractical
combo "$lyd" "$lovsang" cannot
combo "$bilde" "$lovsang" cannot

for name in "Kari Test" "Ola Prøve" "Mona Eksempel" "Pia Demo" "Ingrid Prøve" "Njål Eksempel" "Sunniva Test" "Jon Demo"; do
  code=$(api POST /api/orgs/test/people "{\"name\":\"$name\"}")
  case "$code" in 201|409) ;; *) echo "seed: personen $name ble avvist ($code): $(cat /tmp/seed-body.$$)"; exit 1 ;; esac
done

code=$(api POST /api/orgs/test/day-types '{"name":"Gudstjeneste"}')
case "$code" in 201|409) ;; *) echo "seed: dagtypen ble avvist ($code): $(cat /tmp/seed-body.$$)"; exit 1 ;; esac
dt=$(curl -s "$url/api/orgs/test/day-types" | sed -n 's/.*"id":\([0-9]*\),"name":"Gudstjeneste".*/\1/p')
[ -n "$dt" ] || { echo "seed: fant ikke dagtypen Gudstjeneste"; exit 1; }
code=$(api PUT "/api/orgs/test/day-types/$dt/tasks" "{\"tasks\":[{\"task_id\":$lovsang,\"slots\":1},{\"task_id\":$lyd,\"slots\":1},{\"task_id\":$bilde,\"slots\":1}]}")
[ "$code" = 200 ] || { echo "seed: dagtypens oppgaver ble avvist ($code): $(cat /tmp/seed-body.$$)"; exit 1; }

code=$(api POST /api/orgs/test/plans '{"name":"Høst 2026"}')
case "$code" in 201|409) ;; *) echo "seed: planen ble avvist ($code): $(cat /tmp/seed-body.$$)"; exit 1 ;; esac
code=$(api POST /api/orgs/test/plans/host-2026/events/bulk "{\"from\":\"2026-08-01\",\"to\":\"2026-12-20\",\"weekdays\":[7],\"day_type_id\":$dt}")
[ "$code" = 201 ] || { echo "seed: hendelsene ble avvist ($code): $(cat /tmp/seed-body.$$)"; exit 1; }

# Hva folk kan stille på (svar per person): nok til at «Planlegg» gir en plan.
people=$(curl -s "$url/api/orgs/test/people")
pid() { printf '%s' "$people" | sed -n "s/.*\"id\":\([0-9]*\),\"name\":\"$1\".*/\1/p"; }
choose() { # choose <person> <oppgave-id> <nivå>
  id=$(pid "$1"); [ -n "$id" ] || { echo "seed: fant ikke personen $1"; exit 1; }
  code=$(api PUT "/api/orgs/test/people/$id/tasks/$2" "{\"level\":\"$3\"}")
  [ "$code" = 204 ] || grep -q 'nothing to change' /tmp/seed-body.$$ || { echo "seed: svaret for $1 ble avvist ($code): $(cat /tmp/seed-body.$$)"; exit 1; }  # uendret svar er ikke en feil
}
choose "Kari Test" "$lyd" neutral;      choose "Kari Test" "$bilde" good
choose "Ola Prøve" "$lyd" good;         choose "Ola Prøve" "$lovsang" neutral
choose "Mona Eksempel" "$lyd" neutral;  choose "Mona Eksempel" "$lovsang" good
choose "Pia Demo" "$bilde" neutral
choose "Ingrid Prøve" "$bilde" neutral; choose "Ingrid Prøve" "$lovsang" good
choose "Njål Eksempel" "$lovsang" neutral; choose "Njål Eksempel" "$lyd" poor
choose "Sunniva Test" "$lovsang" neutral;  choose "Sunniva Test" "$bilde" good
choose "Jon Demo" "$lyd" neutral;       choose "Jon Demo" "$lovsang" good

echo "seed: ok. $url/o/test: Lovsang, Lyd, Bilde (Lyd+Bilde impractical, begge kan ikke med Lovsang), 8 personer med oppgavevalg, plan Høst 2026 med 21 søndager (trykk «Planlegg»)"
