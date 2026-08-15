#!/bin/sh
set -eu

archive="${1:-}"
if [ -z "$archive" ] || [ ! -f "$archive" ]; then
  printf '%s\n' "Usage: $0 /exact/path/backup.sql.gz" >&2
  exit 2
fi
gzip -t "$archive"

# backup-postgres.sh writes a checksum next to every archive. Nothing used to
# read it, so the sidecar suggested an integrity guarantee it did not provide.
if [ -f "$archive.sha256" ]; then
  # Compare the digests themselves. The sidecar records whatever path the backup
  # was written with, so `sha256sum -c` only works from that same directory.
  if command -v shasum >/dev/null 2>&1; then actual=$(shasum -a 256 "$archive"); else actual=$(sha256sum "$archive"); fi
  actual=${actual%% *}
  recorded=$(cut -d' ' -f1 < "$archive.sha256")
  if [ "$actual" = "$recorded" ]; then
    printf '%s\n' "Checksum matches the recorded sha256."
  else
    printf 'Backup verification failed: archive does not match %s.sha256\n' "$archive" >&2
    printf '  recorded %s\n  actual   %s\n' "$recorded" "$actual" >&2
    exit 1
  fi
else
  printf '%s\n' "Note: no $archive.sha256 beside the archive, integrity not checked."
fi
verification_db="tappix_restore_verify_$(date -u +%Y%m%d%H%M%S)_$$"
username="${POSTGRES_USER:-tappix}"
cleanup() { docker compose exec -T postgres dropdb --if-exists -U "$username" "$verification_db" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
docker compose exec -T postgres createdb -U "$username" "$verification_db"
gzip -dc "$archive" | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$username" "$verification_db" >/dev/null
tables=$(docker compose exec -T postgres psql -U "$username" -d "$verification_db" -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")
if [ "$tables" -lt 1 ]; then printf '%s\n' "Backup verification failed: restored database is empty" >&2; exit 1; fi
source_tables=$(docker compose exec -T postgres psql -U "$username" -d "${POSTGRES_DB:-tappix}" -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")
[ "$tables" = "$source_tables" ] || { printf 'Backup verification failed: source has %s tables, restore has %s\n' "$source_tables" "$tables" >&2; exit 1; }
for table in companies customers users sales_transactions customer_events audit_logs; do
  source_count=$(docker compose exec -T postgres psql -U "$username" -d "${POSTGRES_DB:-tappix}" -Atc "SELECT count(*) FROM $table")
  restored_count=$(docker compose exec -T postgres psql -U "$username" -d "$verification_db" -Atc "SELECT count(*) FROM $table")
  # The archive restored; these counts are compared against the live database,
  # which has moved on if the backup is not the most recent one. Say so, so an
  # operator restoring an older archive does not read this as a broken file.
  [ "$source_count" = "$restored_count" ] || {
    printf 'Restore succeeded, but %s differs from the live database (live %s, archive %s).\n' "$table" "$source_count" "$restored_count" >&2
    printf '%s\n' "If this archive is not the most recent backup, that is expected: the archive itself unpacked and loaded cleanly." >&2
    exit 1
  }
done
printf 'Backup verification passed: %s tables and critical row counts restored into isolated database %s\n' "$tables" "$verification_db"
