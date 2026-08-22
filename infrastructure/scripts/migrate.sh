#!/bin/sh
set -eu

compose="docker compose"
db_user="${POSTGRES_USER:-tappix}"
db_name="${POSTGRES_DB:-tappix}"

$compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$db_user" -d "$db_name" <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version varchar(32) PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

# Baseline databases created before the migration ledger was introduced.
has_legacy=$($compose exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "SELECT to_regclass('public.companies') IS NOT NULL AND NOT EXISTS(SELECT 1 FROM schema_migrations)")
if [ "$has_legacy" = "t" ]; then
  for file in apps/api/migrations/*.up.sql; do
    version=$(basename "$file" | cut -d_ -f1)
    version_number=$(printf '%s' "$version" | sed 's/^0*//')
    [ "${version_number:-0}" -le 16 ] || continue
    $compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$db_user" -d "$db_name" -c "INSERT INTO schema_migrations(version) VALUES('$version') ON CONFLICT DO NOTHING" >/dev/null
  done
fi

for file in apps/api/migrations/*.up.sql; do
  version=$(basename "$file" | cut -d_ -f1)
  applied=$($compose exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version='$version')")
  [ "$applied" = "t" ] && continue
  echo "Applying $version: $(basename "$file")"
  $compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$db_user" -d "$db_name" < "$file"
  $compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$db_user" -d "$db_name" -c "INSERT INTO schema_migrations(version) VALUES('$version')" >/dev/null
  if [ "$version" = "000006" ] && [ "${TAPPIX_SEED_DEMO:-}" = "1" ]; then
    APP_ENV="${APP_ENV:-development}" TAPPIX_SEED_DEMO=1 sh infrastructure/scripts/seed-demo.sh
  fi
done

echo "Database migrations are up to date."
