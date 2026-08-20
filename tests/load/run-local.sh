#!/bin/sh
set -eu
: "${TAPPIX_TEST_ENV:=1}"
: "${DATABASE_URL:?set local DATABASE_URL}"
: "${TAPPIX_TEST_BASE_URL:=http://localhost:8080}"
MANIFEST="${1:-/tmp/tappix-load-manifest.json}"
USERS="${USERS:-10}"
rm -f "$MANIFEST.tmp" "$MANIFEST"
TAPPIX_TEST_ENV=1 TAPPIX_TEST_BASE_URL="$TAPPIX_TEST_BASE_URL" go run ./apps/api/cmd/testseed -users "$USERS" -companies 1 -manifest "$MANIFEST" >/dev/null
test -s "$MANIFEST"
TAPPIX_TEST_ENV=1 go run ./apps/api/cmd/loadvalidate -manifest "$MANIFEST"
TAPPIX_TEST_ENV=1 go run ./apps/api/cmd/loadrun -manifest "$MANIFEST" -users "$USERS" -iterations "${ITERATIONS:-10}"
