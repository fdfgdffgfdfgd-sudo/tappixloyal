#!/bin/sh
set -eu

: "${TAPPIX_TEST_ENV:?set TAPPIX_TEST_ENV=1}"
[ "$TAPPIX_TEST_ENV" = 1 ] || { echo 'benchmark requires TAPPIX_TEST_ENV=1' >&2; exit 1; }
: "${BENCHMARK_DATABASE_URL:?set BENCHMARK_DATABASE_URL explicitly}"
case "$BENCHMARK_DATABASE_URL" in
  *localhost*/*tappix_bench*|*127.0.0.1*/*tappix_bench*|*postgres*/*tappix_bench*) : ;;
  *) echo 'refusing non-local/non-benchmark database URL' >&2; exit 1;;
esac

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
psql_cmd() {
  if command -v psql >/dev/null 2>&1; then psql "$BENCHMARK_DATABASE_URL" "$@"; else
    docker compose exec -T postgres psql -U tappix -d tappix_bench "$@"
  fi
}
psql_cmd -v ON_ERROR_STOP=1 <<'SQL'
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO public;
SQL

for migration in "$ROOT_DIR"/apps/api/migrations/*.up.sql; do
  echo "applying $migration"
  psql_cmd -v ON_ERROR_STOP=1 -f - < "$migration" >/dev/null
done
echo 'benchmark database reset and migrations applied'
