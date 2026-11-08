#!/bin/sh
# Testdata til en kjørende app: organisasjonen «Test» med oppgavene Lovsang, Lyd og Bilde.
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
echo "seed: ok. $url/o/test har oppgavene Lovsang, Lyd og Bilde"
