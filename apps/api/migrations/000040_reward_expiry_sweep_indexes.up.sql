-- The reward expiry worker looks across every company, so the existing
-- customer_rewards_expiry_idx cannot serve it: that index leads with company_id.
-- Without these the sweep scans the whole table every few minutes.
CREATE INDEX IF NOT EXISTS customer_rewards_reservation_sweep_idx ON customer_rewards(reserved_until) WHERE status='reserved';
CREATE INDEX IF NOT EXISTS customer_rewards_validity_sweep_idx ON customer_rewards(expires_at) WHERE status IN ('available','reserved');
