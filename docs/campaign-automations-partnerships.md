# Campaign automations and partnerships

## Trigger automations

Every active company has three idempotent automation definitions:

- `birthday_bonus` credits the configured points once per calendar year and sends a birthday message;
- `bonus_expiry_3d` groups FIFO lots expiring on the configured day and sends one reminder;
- `winback_30d` sends once for each inactivity cycle, using the latest non-reversed visit or paid receipt as the activity boundary.

The hourly worker records every attempt in `campaign_automation_runs`. Missing email addresses and unavailable channels are recorded as `skipped`; transport errors are recorded as `failed`. Owners can edit templates and settings or force a safe rerun through the API.

## Cross-marketing partnerships

A partnership is active only after the invited company accepts it. Offers are directional: `source_company_id` qualifies the purchase, while `reward_company_id` owns the bonus liability. This allows Coffee Shop → Barbershop and a separate Barbershop → Coffee Shop offer under one partnership.

Redemption requires an eligible canonical source receipt and a customer already registered in the reward tenant. Tappix does not silently copy customer identity between businesses. Awarded points create a normal bonus ledger entry and FIFO lot in the reward company. Per-customer, total and idempotency limits prevent duplicate claims.
