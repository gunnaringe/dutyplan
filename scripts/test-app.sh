#!/bin/sh
# go vet + go test for the app, quiet: one line on success, the whole output on failure.
set -u
cd "$(dirname "$0")/.."
out=$(go vet ./... 2>&1 && go test -count=1 "$@" ./... 2>&1) || { echo "$out"; exit 1; }
echo "app: ok ($(echo "$out" | grep -c '^ok'))"
