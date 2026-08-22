#!/bin/sh
set -eu

[ "${APP_ENV:-}" = "production" ] || { echo "APP_ENV=production is required" >&2; exit 2; }
[ -n "${DATABASE_URL:-}" ] || { echo "DATABASE_URL is required" >&2; exit 2; }
case "$DATABASE_URL" in *sslmode=require*|*sslmode=verify-*) ;; *) echo "Production DATABASE_URL must require TLS" >&2; exit 2;; esac

psql_run() { docker run --rm -i -e DATABASE_URL postgres:17-alpine psql "$DATABASE_URL" -v ON_ERROR_STOP=1 "$@"; }
psql_run <<'SQL'
DO $$ BEGIN
  IF to_regclass('public.companies') IS NOT NULL AND to_regclass('public.schema_migrations') IS NULL THEN
    RAISE EXCEPTION 'untracked legacy schema refused in production';
  END IF;
END $$;
CREATE TABLE IF NOT EXISTS schema_migrations (
  version varchar(32) PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

for file in apps/api/migrations/*.up.sql; do
  version=$(basename "$file" | cut -d_ -f1)
  applied=$(psql_run -Atc "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version='$version')")
  [ "$applied" = "t" ] && continue
  echo "Applying $version: $(basename "$file")"
  { echo 'BEGIN;'; cat "$file"; printf "\nINSERT INTO schema_migrations(version) VALUES('%s');\nCOMMIT;\n" "$version"; } | psql_run
done
echo "Production migrations are up to date: $(psql_run -Atc 'SELECT max(version) FROM schema_migrations')"
