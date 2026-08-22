# Tappix production-readiness matrix

Evidence date: 2026-08-22. `Live verified` means a real external account or delivery was observed; recorded payloads and local services do not qualify.

| Module | Implemented | Functional test | Integration test | Live verified | External blocker |
|---|---:|---:|---:|---:|---|
| Poster | yes | yes | yes | no | Poster account/access token |
| iiko | no | no | no | no | API credentials and product contract |
| Syrve | no | no | no | no | API credentials and product contract |
| Kaspi | generic inbound transaction contract only | yes | yes | no | Merchant account and confirmed Kaspi contract |
| WhatsApp | Cloud API transport/OTP/reports | yes | local failure/retry | no | Meta WABA, approved templates, production number |
| Email | SMTP transport | yes | Mailpit received | no | Production SMTP account/domain |
| Billing | admin-managed lifecycle; no payment gateway | yes | yes | no | Acquirer/payment provider |
| Start plan | yes | yes | yes | n/a | — |
| Pro plan | yes | yes | yes | n/a | — |
| Business plan | yes | yes | yes | n/a | — |
| Trial | yes | yes | yes | n/a | — |
| Analytics | yes | yes | yes | n/a | — |
| Campaigns | yes | yes | yes | email via Mailpit | WhatsApp credentials for live WA delivery |
| Staff | yes | yes | yes | production-like | — |
| Customer registration | yes | yes | yes | production-like | — |
| Customer loyalty card | yes | yes | yes | production-like | — |
| Branches | yes | yes | yes | production-like | — |

## Status rules

- `Implemented`: executable product/backend behavior exists; a card or interface alone is not enough.
- `Functional test`: deterministic behavior is asserted without requiring the real provider.
- `Integration test`: behavior crosses API/database/worker or delivery boundaries in the production-like stack.
- `Live verified`: a real third-party account completed the operation. This cannot be inferred from environment variables.

## Environment configuration

| Variable | Class | Required when | Validation |
|---|---|---|---|
| `APP_ENV` | required | production | activates fail-fast production guards |
| `DATABASE_URL` | required | production/API | present; rejects obvious local/default credentials in production |
| `REDIS_ADDR` | required | production/API | Redis startup ping must pass |
| `JWT_SECRET` | required | production | at least 32 chars; rejects placeholder words |
| `APP_URL` | required | production | HTTPS required |
| `SMTP_HOST`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_TLS` | required | production email | credentials required and TLS must be true |
| `SMTP_PORT`, `SMTP_FROM` | optional | email | defaults only suit local development |
| `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_PHONE_NUMBER_ID` | required | production | non-empty; live validity requires provider verification |
| `WHATSAPP_APP_SECRET`, `WHATSAPP_VERIFY_TOKEN` | required | inbound WA webhooks | secret/signature verification |
| `WHATSAPP_GRAPH_VERSION`, `WHATSAPP_API_BASE` | optional | WhatsApp | safe provider defaults |
| `METRICS_TOKEN` | required | production | prevents public operational metrics access |
| `OTP_DEV_MODE` | development only | local OTP | must be false in production |
| `POSTER_API_BASE_URL` | test/development override | recorded Poster server | real Poster URL is the default |
| `TAPPIX_SEED_DEMO`, `TAPPIX_TEST_ENV` | development/test only | fixtures/load tests | scripts refuse production use |
| `UPLOAD_DIR`, `HTTP_ADDR` | optional | deployment | filesystem/listener overrides |

Demo credentials are stored only in `infrastructure/seeds/demo.sql`, invoked explicitly with `TAPPIX_SEED_DEMO=1`. Production migrations create no companies, users, customers, subscriptions, or known passwords.
