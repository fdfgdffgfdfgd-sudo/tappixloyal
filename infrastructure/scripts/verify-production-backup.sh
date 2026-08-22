#!/bin/sh
set -eu

archive="${1:-}"
[ -f "$archive" ] || { echo "Usage: RESTORE_TEST_DATABASE_URL=... $0 /exact/backup.dump" >&2; exit 2; }
: "${RESTORE_TEST_DATABASE_URL:?A separate empty restore-test database URL is required}"
case "$RESTORE_TEST_DATABASE_URL" in "${DATABASE_URL:-__never__}") echo "Refusing to restore over the source/production database" >&2; exit 2;; esac
[ -f "$archive.sha256" ] || { echo "Missing checksum sidecar" >&2; exit 2; }
if command -v sha256sum >/dev/null 2>&1; then actual=$(sha256sum "$archive"); else actual=$(shasum -a 256 "$archive"); fi
actual=${actual%% *}
recorded=$(cut -d' ' -f1 < "$archive.sha256")
[ "$actual" = "$recorded" ] || { echo "Backup checksum mismatch" >&2; exit 1; }
archive_dir=$(cd "$(dirname "$archive")" && pwd)
archive_name=$(basename "$archive")
docker run --rm -e RESTORE_TEST_DATABASE_URL -v "$archive_dir:/backup:ro" postgres:17-alpine \
  pg_restore --clean --if-exists --no-owner --no-privileges --exit-on-error --dbname="$RESTORE_TEST_DATABASE_URL" "/backup/$archive_name"
tables=$(docker run --rm -e RESTORE_TEST_DATABASE_URL postgres:17-alpine psql "$RESTORE_TEST_DATABASE_URL" -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")
[ "$tables" -gt 0 ] || { echo "Restore verification produced no public tables" >&2; exit 1; }
version=$(docker run --rm -e RESTORE_TEST_DATABASE_URL postgres:17-alpine psql "$RESTORE_TEST_DATABASE_URL" -Atc "SELECT max(version) FROM schema_migrations")
printf 'Production backup restore verified in isolated database: %s tables, migration %s\n' "$tables" "$version"
