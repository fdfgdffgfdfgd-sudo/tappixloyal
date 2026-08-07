#!/bin/sh
set -eu

archive="${1:-}"
if [ -z "$archive" ] || [ ! -f "$archive" ]; then
  printf '%s\n' "Usage: CONFIRM_RESTORE=tappix $0 /exact/path/backup.sql.gz" >&2
  exit 2
fi
if [ "${CONFIRM_RESTORE:-}" != "tappix" ]; then
  printf '%s\n' "Restore replaces database objects. Set CONFIRM_RESTORE=tappix to continue." >&2
  exit 2
fi
database="${POSTGRES_DB:-tappix}"
username="${POSTGRES_USER:-tappix}"
gzip -dc "$archive" | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$username" "$database"
