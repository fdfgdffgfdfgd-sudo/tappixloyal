# Live integration report

Update only from a real production event. Never infer `Live connected` or `Real data verified` from automated tests.

| Provider | Implemented | Automated tested | Production configured | Live connected | Real data verified | Last test | Blocker |
|---|---:|---:|---:|---:|---:|---|---|
| Poster | yes | yes | no | no | no | 2026-08-22 local | Awaiting real Poster account/token |
| iiko | no | no | no | no | no | — | Adapter and real account required |
| Syrve | no | no | no | no | no | — | Adapter and real account required |
| Kaspi | partial canonical inbound contract | yes | no | no | no | 2026-08-22 local | Confirm merchant access and supported API first |
| WhatsApp Cloud API | yes | yes | no | no | no | 2026-08-22 local | Awaiting WABA credentials, approved templates and consented phone |
| Email SMTP | yes | yes via Mailpit | no | no | no | 2026-08-22 local | Awaiting SMTP credentials, mailbox and SPF/DKIM/DMARC access |

For each live Poster run record account/business identity, connection timestamp, first sync counts (`received/processed/created/updated/skipped/failed`), second sync counts, reconciliation result, last/next scheduled sync and direct CRM spot checks. Do not store tokens in this file.
