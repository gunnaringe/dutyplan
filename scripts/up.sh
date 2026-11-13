#!/bin/sh
# Starts the app (the built binary bin/dutyplan) in the foreground. If the database file does not exist yet, the demo
# organisation «Test» is put into it first (scripts/seed-demo.sh), so a first start has something to look at.
# Usage: mise run up [-- --addr 127.0.0.1:8080 --db file.db]   (the arguments go to the app)
set -eu

db=dutyplan.db
prev=
for arg in "$@"; do
  [ "$prev" = --db ] && db=$arg
  case "$arg" in --db=*) db=${arg#--db=} ;; esac
  prev=$arg
done

if [ -e "$db" ]; then
  exec bin/dutyplan "$@"
fi

out=$(mktemp)
bin/dutyplan "$@" >"$out" &
pid=$!
# A background job in a script ignores Ctrl-C, so pass the stop on to the app, which shuts down cleanly on SIGTERM.
trap 'kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; rm -f "$out"; exit 0' INT TERM

line=
for _ in $(seq 1 100); do
  line=$(sed -n '1p' "$out")
  [ -n "$line" ] && break
  kill -0 "$pid" 2>/dev/null || { echo "up: the app stopped before it was ready"; rm -f "$out"; exit 1; }
  sleep 0.1
done
[ -n "$line" ] || { echo "up: the app did not say where it listens"; kill "$pid" 2>/dev/null || true; rm -f "$out"; exit 1; }
echo "$line"

if sh scripts/seed-demo.sh "${line#dutyplan: }" >/dev/null; then
  echo "up: new database $db with the demo organisation «Test» (${line#dutyplan: }/o/test)"
else
  echo "up: the demo data could not be put in; the app runs with an empty database"
fi

rc=0
wait "$pid" || rc=$?
rm -f "$out"
exit "$rc"
