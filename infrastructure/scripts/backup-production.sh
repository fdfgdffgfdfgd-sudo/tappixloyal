#!/bin/sh
set -eu

[ "${APP_ENV:-}" = "production" ] || { echo "APP_ENV=production is required" >&2; exit 2; }
: "${DATABASE_URL:?DATABASE_URL is required}"
backup_dir="${BACKUP_DIR:-/var/backups/tappix}"
retention_days="${BACKUP_RETENTION_DAYS:-14}"
timestamp=$(date -u +%Y%m%dT%H%M%SZ)
umask 077
mkdir -p "$backup_dir"
target="$backup_dir/tappix-$timestamp.dump"
docker run --rm -e DATABASE_URL -v "$backup_dir:/backup" postgres:17-alpine \
  pg_dump "$DATABASE_URL" --format=custom --no-owner --no-privileges --file="/backup/$(basename "$target")"
test -s "$target"
if command -v sha256sum >/dev/null 2>&1; then sha256sum "$target" > "$target.sha256"; else shasum -a 256 "$target" > "$target.sha256"; fi
find "$backup_dir" -type f \( -name 'tappix-*.dump' -o -name 'tappix-*.dump.sha256' \) -mtime "+$retention_days" -delete
printf '%s\n' "$target"
