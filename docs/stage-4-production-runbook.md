# Tappix Stage 4 production runbook

This runbook prepares and operates a real deployment. It does not claim that a server or provider is live until the evidence table is filled from that environment.

## Current status

| Area | Status | Evidence / blocker |
|---|---|---|
| Production deployment | READY FOR SERVER ACCESS | No host, DNS zone, registry access or approved domains were supplied. |
| Poster | READY FOR CREDENTIALS | Adapter and automated tests exist; no real Poster token/account supplied. |
| iiko | BLOCKED | No implemented adapter and no real account. |
| Syrve | BLOCKED | No implemented adapter and no real account. |
| Kaspi | READY FOR ACCESS DISCOVERY | Only the canonical inbound contract exists; merchant API/access is unconfirmed. |
| WhatsApp | READY FOR CREDENTIALS | Cloud API transport exists; no WABA credentials or destination phone supplied. |
| Email | READY FOR CREDENTIALS + DNS | SMTP works against Mailpit; no production provider or domain DNS access supplied. |
| Pilot | NOT STARTED | Pilot business and consented participants not supplied. |

## Environment boundaries

- Development uses `docker-compose.yml`, local Postgres/Redis/Mailpit, and may seed fixtures only with `TAPPIX_SEED_DEMO=1`.
- CI creates disposable volumes and explicitly enables fixtures.
- Production uses `docker-compose.production.yml` and `.env.production`. It never starts Mailpit, never runs `seed-demo.sh`, and expects separate TLS-enabled PostgreSQL and authenticated Redis endpoints.
- Never copy a development database or `.env` to production. Keep `.env.production` mode `0600`; it is gitignored.
- Use separate provider applications/credentials and separate database/Redis instances for staging if staging is introduced.

## Before first deployment

1. Create an Ubuntu/Debian host with Docker Engine + Compose, firewall ports 22/80/443, and enough encrypted disk for uploads and monitoring.
2. Point the approved application domain to the host. `APP_DOMAIN` is configuration, never compiled into the application.
3. Provision PostgreSQL with TLS and automated provider snapshots, and Redis with authentication on private networking. Do not expose either to the Internet.
4. Copy `.env.production.example` to `.env.production`, replace all `REQUIRED_*` values, set mode `0600`, and use immutable image tags produced by `Build production images`.
5. Create `infrastructure/secrets/metrics_token` from exactly `METRICS_TOKEN`, mode `0600`. It is gitignored.
6. Generate independent secrets, for example `openssl rand -hex 32`; never reuse JWT, database, Redis, integration or metrics secrets.
7. Leave all SMTP fields blank together and all WhatsApp fields blank together until credentials exist. A partially configured provider fails startup.

Core required values: `APP_ENV`, `APP_DOMAIN`, `APP_URL`, `API_URL`, `WEB_ORIGIN`, `ACME_EMAIL`, `DATABASE_URL`, `REDIS_ADDR`, `REDIS_PASSWORD`, `JWT_SECRET`, `INTEGRATION_ENCRYPTION_KEY`, `METRICS_TOKEN`, `RELEASE_SHA`, `TAPPIX_API_IMAGE`, `TAPPIX_WEB_IMAGE`.

## Deploy

```sh
git fetch origin main
git checkout --detach <approved-sha>
cp /secure/location/tappix.production.env .env.production
chmod 600 .env.production infrastructure/secrets/metrics_token
sh infrastructure/scripts/deploy-production.sh .env.production
```

The script refuses dirty code, mutable `latest` images, release/SHA mismatch, missing metrics secret, non-production mode and non-TLS PostgreSQL. It applies each migration transactionally before changing application containers, starts API/web, waits for internal health, then enables Caddy and verifies public HTTPS. Caddy handles HTTP-to-HTTPS and ACME renewal automatically.

After deployment record:

```sh
curl -fsS https://$APP_DOMAIN/health
docker compose --env-file .env.production -f docker-compose.production.yml ps
docker compose --env-file .env.production -f docker-compose.production.yml logs --since=10m api
```

`/health` reports PostgreSQL, Redis, workers, migration version and `RELEASE_SHA`. It returns 503 if a required dependency or workers are unavailable. Production logs are structured JSON at INFO/WARN/ERROR; DEBUG is suppressed in production. Request logs use normalized routes and should not contain request bodies or tokens.

## First superadmin

After migrations, create exactly one unpredictable administrator. The bootstrap refuses demo passwords, weak passwords, non-TLS databases and a second active superadmin.

```sh
set -a; . ./.env.production; set +a
export BOOTSTRAP_ADMIN_EMAIL='<approved operator email>'
export BOOTSTRAP_ADMIN_PASSWORD='<unique password from password manager>'
sh infrastructure/scripts/bootstrap-superadmin.sh
unset BOOTSTRAP_ADMIN_PASSWORD
```

Enable MFA immediately. Create the pilot owner through the normal invitation/onboarding flow; never seed a company in production.

## Monitoring and daily operations

Prometheus is private and scrapes authenticated `/metrics`. Alerts cover API down, repeated 5xx, workers not ready and integration failures. Connect the existing organization alert receiver/Grafana after server access; no public dashboard is exposed by this stack.

Daily operator check:

```sh
set -a; . ./.env.production; set +a
sh infrastructure/scripts/operational-check.sh
```

Also review Caddy certificate expiry/renewal logs, Prometheus active alerts, recent integration history and support questions. Do not send customer payloads to an error tracker. No Sentry project was present or supplied, so Sentry is not marked configured.

## Backups and restore drill

Install the provided systemd unit/timer, then verify it is active:

```sh
sudo cp infrastructure/systemd/tappix-backup.* /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now tappix-backup.timer
sudo systemctl start tappix-backup.service
sudo systemctl status tappix-backup.service tappix-backup.timer
```

Backups are custom-format PostgreSQL dumps, mode `0600`, SHA-256 checked, daily with 14-day default local retention. Configure encrypted off-host replication or provider object storage before calling disaster recovery ready. At least monthly, restore the latest dump into a separate temporary database; never test restore over production:

```sh
RESTORE_TEST_DATABASE_URL='postgres://.../tappix_restore_test?sslmode=require' \
  sh infrastructure/scripts/verify-production-backup.sh /var/backups/tappix/<latest.dump>
```

## Rollback

Keep `.env.production.rollback` with the previous immutable image tags and SHA. Application rollback is explicit:

```sh
CONFIRM_ROLLBACK=tappix sh infrastructure/scripts/rollback-production.sh .env.production.rollback
```

Migrations are not reversed automatically. Only additive/backward-compatible migrations should ship during the pilot. If a migration is incompatible, stop deployment and restore into a separate database before any production decision.

## Email domain checklist

- Verify provider ownership and approved From address/name.
- Publish provider-specific SPF and DKIM records.
- Publish DMARC first in monitoring mode, review reports, then strengthen policy.
- Verify subject, HTML/plain-text body, links, mobile rendering and delivery to a real mailbox.
- Record the DNS check time and message/provider ID. Until then status remains `READY FOR CREDENTIALS + DNS`.

## Release record

For every release record: Git SHA, immutable API/web image digests, deployment UTC time, maximum migration version, health response, latest successful backup UTC time, active alerts, operator and rollback image tags.
