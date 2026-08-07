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
printf '%s\n' "$target"
