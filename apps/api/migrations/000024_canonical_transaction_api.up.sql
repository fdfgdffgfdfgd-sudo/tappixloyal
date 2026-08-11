ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS scopes varchar(80)[] NOT NULL DEFAULT ARRAY['legacy'],
  ADD COLUMN IF NOT EXISTS sandbox boolean NOT NULL DEFAULT false;

ALTER TABLE sales_transactions
  ADD COLUMN IF NOT EXISTS sandbox boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS refund_reason text;
CREATE INDEX IF NOT EXISTS sales_transactions_environment_idx
  ON sales_transactions(company_id,sandbox,occurred_at DESC);

ALTER TABLE customer_events ADD COLUMN IF NOT EXISTS sandbox boolean NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS customer_events_environment_idx
  ON customer_events(company_id,sandbox,event_type,occurred_at DESC);

ALTER TABLE integration_jobs
  ADD COLUMN IF NOT EXISTS retried_by uuid REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS retried_at timestamptz;

ALTER TABLE visits
  ADD COLUMN IF NOT EXISTS reversed_at timestamptz,
  ADD COLUMN IF NOT EXISTS reversal_transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS reversed_points integer NOT NULL DEFAULT 0 CHECK(reversed_points >= 0);

UPDATE api_keys SET scopes=ARRAY['transactions.read','transactions.write','transactions.refund','jobs.retry']
WHERE scopes=ARRAY['legacy']::varchar[];
