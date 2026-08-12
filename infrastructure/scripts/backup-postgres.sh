#!/bin/sh
set -eu

backup_dir="${BACKUP_DIR:-./backups}"
database="${POSTGRES_DB:-tappix}"
username="${POSTGRES_USER:-tappix}"
timestamp=$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$backup_dir"
target="$backup_dir/tappix-$timestamp.sql.gz"

docker compose exec -T postgres pg_dump --clean --if-exists --no-owner --no-privileges -U "$username" "$database" | gzip -9 > "$target"
test -s "$target"
gzip -t "$target"
if command -v shasum >/dev/null 2>&1; then shasum -a 256 "$target" > "$target.sha256"; else sha256sum "$target" > "$target.sha256"; fi
printf '%s\n' "$target"
