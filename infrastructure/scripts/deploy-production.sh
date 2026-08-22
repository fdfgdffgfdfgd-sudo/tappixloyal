#!/bin/sh
set -eu

env_file="${1:-.env.production}"
[ -f "$env_file" ] || { echo "Missing $env_file; copy .env.production.example and fill it" >&2; exit 2; }
[ "$(git status --porcelain)" = "" ] || { echo "Refusing deployment from a dirty worktree" >&2; exit 2; }
set -a
. "$env_file"
set +a
[ "${APP_ENV:-}" = "production" ] || { echo "APP_ENV=production is required" >&2; exit 2; }
current_sha=$(git rev-parse HEAD)
[ "${RELEASE_SHA:-}" = "$current_sha" ] || { echo "RELEASE_SHA must equal checked-out commit $current_sha" >&2; exit 2; }
: "${TAPPIX_WEB_IMAGE:?TAPPIX_WEB_IMAGE must be an immutable image reference}"
: "${TAPPIX_API_IMAGE:?TAPPIX_API_IMAGE must be an immutable image reference}"
: "${API_URL:?API_URL is required}"
: "${REDIS_PASSWORD:?REDIS_PASSWORD is required}"
case "$DATABASE_URL$REDIS_PASSWORD$JWT_SECRET$INTEGRATION_ENCRYPTION_KEY$METRICS_TOKEN$TAPPIX_WEB_IMAGE$TAPPIX_API_IMAGE" in *REQUIRED*|*Admin2026\!*|*Tappix2026\!*|*DocMed2026\!*) echo "Placeholder or demo credential refused" >&2; exit 2;; esac
case "$TAPPIX_WEB_IMAGE$TAPPIX_API_IMAGE" in *:latest*) echo "The mutable :latest tag is forbidden" >&2; exit 2;; esac
[ -s infrastructure/secrets/metrics_token ] || { echo "Create chmod 600 infrastructure/secrets/metrics_token" >&2; exit 2; }
[ "$(cat infrastructure/secrets/metrics_token)" = "$METRICS_TOKEN" ] || { echo "Prometheus token file does not match METRICS_TOKEN" >&2; exit 2; }

APP_ENV=production DATABASE_URL="$DATABASE_URL" sh infrastructure/scripts/migrate-production.sh
docker compose --env-file "$env_file" -f docker-compose.production.yml pull
docker compose --env-file "$env_file" -f docker-compose.production.yml up -d api web prometheus
for attempt in $(seq 1 30); do
  docker compose --env-file "$env_file" -f docker-compose.production.yml exec -T api wget -q -O /dev/null http://127.0.0.1:8080/health && break
  [ "$attempt" -lt 30 ] || { docker compose --env-file "$env_file" -f docker-compose.production.yml logs api; exit 1; }
  sleep 2
done
docker compose --env-file "$env_file" -f docker-compose.production.yml up -d caddy
curl -fsS "https://${APP_DOMAIN}/health" >/dev/null
printf 'Deployed release %s at %s\n' "$current_sha" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
