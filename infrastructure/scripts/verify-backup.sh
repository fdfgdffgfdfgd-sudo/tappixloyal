#!/bin/sh
set -eu

archive="${1:-}"
if [ -z "$archive" ] || [ ! -f "$archive" ]; then
  printf '%s\n' "Usage: $0 /exact/path/backup.sql.gz" >&2
  exit 2
fi
gzip -t "$archive"
verification_db="tappix_restore_verify_$(date -u +%Y%m%d%H%M%S)_$$"
username="${POSTGRES_USER:-tappix}"
cleanup() { docker compose exec -T postgres dropdb --if-exists -U "$username" "$verification_db" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
docker compose exec -T postgres createdb -U "$username" "$verification_db"
gzip -dc "$archive" | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$username" "$verification_db" >/dev/null
tables=$(docker compose exec -T postgres psql -U "$username" -d "$verification_db" -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")
if [ "$tables" -lt 1 ]; then printf '%s\n' "Backup verification failed: restored database is empty" >&2; exit 1; fi
printf 'Backup verification passed: %s tables restored into isolated database %s\n' "$tables" "$verification_db"
