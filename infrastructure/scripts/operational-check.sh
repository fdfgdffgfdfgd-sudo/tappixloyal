#!/bin/sh
set -eu

: "${APP_URL:?APP_URL is required}"
: "${DATABASE_URL:?DATABASE_URL is required}"
: "${REDIS_ADDR:?REDIS_ADDR is required}"
curl -fsS "$APP_URL/health"
docker run --rm -e DATABASE_URL postgres:17-alpine pg_isready -d "$DATABASE_URL"
redis_host=${REDIS_ADDR%:*}; redis_port=${REDIS_ADDR##*:}
if [ -n "${REDIS_PASSWORD:-}" ]; then
  docker run --rm --network host redis:7-alpine redis-cli -h "$redis_host" -p "$redis_port" -a "$REDIS_PASSWORD" ping
else
  docker run --rm --network host redis:7-alpine redis-cli -h "$redis_host" -p "$redis_port" ping
fi
if [ -n "${BACKUP_DIR:-}" ]; then
  latest=$(find "$BACKUP_DIR" -type f -name 'tappix-*.dump' -mtime -2 -print -quit)
  [ -n "$latest" ] || { echo "No production backup newer than 48 hours" >&2; exit 1; }
  echo "Latest backup: $latest"
fi
