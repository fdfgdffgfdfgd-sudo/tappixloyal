#!/bin/sh
set -eu

if [ "${TAPPIX_SEED_DEMO:-}" != "1" ]; then
  echo "Refusing to seed demo data: set TAPPIX_SEED_DEMO=1" >&2
  exit 2
fi
if [ "${APP_ENV:-development}" = "production" ]; then
  echo "Refusing to seed demo data in production" >&2
  exit 2
fi

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 \
  -U "${POSTGRES_USER:-tappix}" -d "${POSTGRES_DB:-tappix}" \
  < infrastructure/seeds/demo.sql
echo "Development demo fixtures are ready."
