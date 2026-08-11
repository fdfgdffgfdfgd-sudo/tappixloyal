# Business data foundation

Migration `000022_business_data_foundation` introduces the canonical data layer used by POS integrations, revenue analytics, campaign attribution and referrals.

## Sources of truth

- `sales_transactions` and its items/payments are the financial source of truth. A visit is not a substitute for a receipt.
- `bonus_ledger` remains the source of truth for customer point balances. Its optional `sales_transaction_id` links every purchase-related credit or debit to the originating receipt.
- `customer_events` is the append-only customer journey stream used for funnels.
- `customer_attributions` stores resolved first-touch, last-touch, registration and transaction attribution.
- `analytics_daily_facts` and `analytics_customer_features` are rebuildable projections, never transactional sources of truth.

## Transaction ingestion

Every provider payload is normalized into `sales_transactions`, `sales_transaction_items` and `payments`. Duplicate delivery is safe when adapters preserve the provider receipt identifier:

```text
UNIQUE(company_id, provider, external_id)
```

Commands generated inside Tappix should additionally set `idempotency_key`. Refunds and cancellations are represented as separate or updated transaction records linked through `original_transaction_id`; ledger history must not be overwritten.

## Integration lifecycle

`integration_connections` contains public connection metadata and encrypted credential bytes. Plaintext credentials must never be persisted in `config` or logs.

`integration_location_mappings` maps a provider location to a Tappix branch. `integration_sync_cursors` records provider watermarks. Work is processed through `integration_jobs`; exhausted or malformed records are exposed through `integration_failures` for reconciliation.

## Customer journey

The expected funnel event types are:

```text
smart_link_opened
registration_started
customer_registered
first_purchase_completed
second_purchase_completed
reward_earned
reward_redeemed
```

Anonymous events use `anonymous_id`. After registration, identity resolution may associate subsequent events with `customer_id`; historical rows remain immutable.

## Campaigns and referrals

`campaign_conversions` records delivery, engagement and purchase outcomes. Revenue conversions reference the canonical transaction.

Referral state progresses as follows:

```text
clicked -> registered -> qualified -> reward_pending -> rewarded
                                      \-> rejected
rewarded -> reversed
```

`referral_rewards` stores separate benefits for referrer and friend. Its idempotency key prevents repeated payout when a POS webhook or background job is retried.

## Tenant isolation

All new business tables contain `company_id`, have PostgreSQL row-level security enabled and use the existing `app.company_id` tenant policy. Services must still filter by `company_id`; RLS is the second isolation layer.

## Next implementation boundary

This migration intentionally adds no public API. The next layer should introduce a provider-neutral ingestion service that validates tenant ownership, normalizes amounts and currency, upserts a receipt by provider identity, writes bonus operations and outbox events in one database transaction, and then schedules analytics projection updates.
