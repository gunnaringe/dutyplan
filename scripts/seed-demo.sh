#!/bin/sh
# Testdata til en kjørende app: organisasjonen «Test» med oppgavene Lovsang, Lyd og Bilde, og kombinasjonene
# Lyd + Bilde = upraktisk (bør ikke), Lyd + Lovsang og Bilde + Lovsang = kan ikke kombineres.
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
combo "$lyd" "$bilde" upraktisk
combo "$lyd" "$lovsang" kan_ikke
combo "$bilde" "$lovsang" kan_ikke
echo "seed: ok. $url/o/test har Lovsang, Lyd og Bilde; Lyd+Bilde upraktisk, begge kan ikke kombineres med Lovsang"
