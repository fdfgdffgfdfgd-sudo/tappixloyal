#!/bin/sh
set -eu

env_file="${1:-.env.production.rollback}"
[ -f "$env_file" ] || { echo "Provide an env file containing the previous immutable image tags and RELEASE_SHA" >&2; exit 2; }
echo "Rollback never reverses migrations automatically. Confirm the previous application version is forward-compatible with the current schema."
[ "${CONFIRM_ROLLBACK:-}" = "tappix" ] || { echo "Set CONFIRM_ROLLBACK=tappix to continue" >&2; exit 2; }
docker compose --env-file "$env_file" -f docker-compose.production.yml up -d api web
docker compose --env-file "$env_file" -f docker-compose.production.yml ps
