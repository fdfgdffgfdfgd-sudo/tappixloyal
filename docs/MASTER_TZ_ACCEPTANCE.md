# Tappix Master ТЗ 1–78 — acceptance evidence

Актуально для ветки `agent/master-tz-completion`. Этот документ фиксирует не обещания, а проверяемые точки приёмки.

## Product (разделы 1–50)

- Единый admin shell, навигация, responsive layout и design tokens: `SectionShell`, `ui-system`, `design-system-v6.css`, `design-system/tappix/MASTER.md`.
- Loyalty flow: понятный выбор сценария, настройка штамп-карты, preview, publish summary и canonical reward rule: `program-mechanics.tsx`, `PATCH /settings/guest-portal`, migration 32.
- Referral flow: объясняющая воронка, базовые награды, advanced anti-fraud, live summary, analytics и пример ссылки: `referrals-page.tsx`, referral API and tables.
- Guest wallet: одна активная механика, следующая награда, QR только по CTA, стабильный 6-digit code, rewards/history and branding: `premium-guest-wallet.tsx`, migration 30.
- Staff Mode: QR и tenant-scoped code lookup, masked preview, visit confirmation and reward actions: `staff-scanner.tsx`.
- Overview, analytics, integrations, subscriptions, reports, onboarding and Platform Admin use the same product language and explicit loading/empty/error/success states.
- Product integration acceptance covers publish → registration → wallet progress → Staff visit → reward issuance → idempotent redemption → guest history.

## Engineering (разделы 37–50, 57–65, 75–78)

- Tenant identity is derived from authenticated JWT/API key context; cross-tenant customer, customer-code and branch reads are asserted as 404.
- RBAC, permission and server-side module entitlement guards wrap protected routes.
- Critical mutations emit audit rows; HTTP and domain telemetry include request, tenant and actor identifiers without credentials or OTP.
- Canonical loyalty state is returned by `/customer/wallet`; frontend does not calculate reward eligibility.
- Migrations 1–33 apply cleanly to an isolated empty database. Customer-code backfill, report delivery and program-engine binding have reversible migrations.
- Unit, integration and browser E2E tests are CI gates. Browser tests cover desktop/mobile, WCAG A/AA, horizontal overflow, Escape, focus trap and focus restoration.
- API contract is maintained in `docs/openapi.yaml`, including cashier lookup, canonical wallet, reports and signed report artifacts.

## Production (разделы 51–78)

- Production config fails fast for unsafe secrets, dev OTP and missing delivery/public URL configuration.
- SMTP and WhatsApp credentials remain backend-only. WhatsApp report documents use expiring HMAC links; outbound report webhooks use registered tenant endpoints and HMAC signatures.
- Report jobs support idempotent scheduling, status, timeout recovery, three automatic attempts, manual retry, audit and CSV/XLSX/PDF artifacts.
- Docker builds run as non-root application users behind Nginx. Health and protected Prometheus metrics are available.
- `npm run db:backup` produces a SHA-256 sidecar; `npm run db:verify-backup -- <file>` restores into an isolated temporary database. Primary restore still requires `CONFIRM_RESTORE=tappix`.
- `.env`, backups, credentials and uploads are ignored by Git.

## Required acceptance commands

```bash
npm run test:api
npm run lint
npm run build
npm run test:integration
npm run test:e2e
npm audit --omit=dev --audit-level=high
docker compose up -d --build
npm run db:migrate
npm run db:backup
npm run db:verify-backup -- /absolute/path/to/backup.sql.gz
```

The GitHub Actions workflow repeats API formatting/tests, lint/build, a fresh Docker integration environment and desktop/mobile browser E2E before merge.
