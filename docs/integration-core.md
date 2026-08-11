# Integration Core

The Integration Core converts provider-specific POS data into the canonical transaction model introduced in migration `000022`.

## Ingestion guarantees

- Provider receipts are deduplicated by `(company_id, provider, external_id)`.
- Receipt, items, payments, visit, bonus ledger operations, customer journey event and outbox event are committed atomically.
- A repeated provider delivery returns the existing transaction with `duplicate: true`.
- Customer matching order is external customer link, then normalized phone. Names are never used for automatic merging.
- Location mappings are explicit; an unknown provider location does not silently map to a branch.
- Completed receipts may create loyalty effects. Preliminary or pending receipts never update balances.

## Inbound webhooks

Each inbound endpoint has an individual secret that is encrypted at rest and displayed once when created. Providers sign:

```text
signature = hex(HMAC-SHA256(secret, timestamp + "." + raw_body))
```

Required headers:

```text
X-Tappix-Timestamp: unix timestamp
X-Tappix-Signature: sha256=<hex signature>
```

The allowed clock difference is five minutes. The raw request body is limited to 2 MiB. Replayed provider event identities are acknowledged without applying the receipt twice.

## Outbound webhooks

Transactional outbox events are fanned out into durable webhook deliveries. Delivery uses the same signature format, has an explicit event ID, does not follow redirects and rejects URLs resolving to local or private addresses.

Failed deliveries use exponential backoff and become `dead` after the configured maximum attempts. Endpoint health moves to `error` after repeated failures. Payloads and truncated responses remain available in the delivery log for reconciliation.

## Adapter boundary

Provider implementations satisfy `integration.POSAdapter`. Adapters only authorize, fetch and normalize provider data. They must not write visits, bonuses, analytics or referrals directly; all mutations go through `integration.Service.Ingest`.

## Canonical transactions API

POS clients use scoped API keys. Available scopes are `transactions.read`, `transactions.write`, `transactions.refund` and `jobs.retry`. Sandbox keys can read and write only sandbox receipts; sandbox ingestion never changes visits, balances, customer events or the transactional outbox.

```text
POST /api/v1/integrations/transactions/quote
POST /api/v1/integrations/transactions
GET  /api/v1/integrations/transactions/{id}
POST /api/v1/integrations/transactions/{id}/refund
POST /api/v1/integration-jobs/{id}/retry
```

Quote is read-only and expires conceptually after five minutes. Closing a receipt recalculates earned points and validates the requested spend instead of trusting values returned by the POS. Refunds lock the original receipt, reject over-refunds, create an immutable reversal receipt and reverse related loyalty effects. Full refunds also reverse the linked visit once.

## Poster read-only import

Poster connections accept an encrypted `accessToken` (or `token`) credential. Manual synchronization queues locations, customers and the previous 90 days of closed transactions in that order. Imported locations remain `unmapped` until an owner maps them to a Tappix branch; receipts from an unmapped location are retained but do not create a loyalty visit.

The worker imports clients in pages, links them by normalized phone and keeps records without a usable phone in `pending`. Receipts always pass through the canonical idempotent ingestion service. Active Poster connections receive a daily one-day reconciliation; missing receipts are re-imported and every run is recorded in `reconciliation_runs`.

Production uses `https://joinposter.com/api`. `POSTER_API_BASE_URL` is reserved for isolated contract tests and controlled staging environments.

Poster webhook URLs contain a one-time generated secret. Incoming payload values are used only to identify the event and transaction; Tappix fetches the authoritative receipt from Poster before ingestion. Closed receipts use canonical idempotency, while return events create an immutable full reversal and safely recover the original receipt first when the close event was missed.
