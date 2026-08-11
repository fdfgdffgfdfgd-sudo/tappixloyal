ALTER TABLE visits DROP COLUMN IF EXISTS reversed_points, DROP COLUMN IF EXISTS reversal_transaction_id, DROP COLUMN IF EXISTS reversed_at;
ALTER TABLE integration_jobs DROP COLUMN IF EXISTS retried_at, DROP COLUMN IF EXISTS retried_by;
DROP INDEX IF EXISTS customer_events_environment_idx;
ALTER TABLE customer_events DROP COLUMN IF EXISTS sandbox;
DROP INDEX IF EXISTS sales_transactions_environment_idx;
ALTER TABLE sales_transactions DROP COLUMN IF EXISTS refund_reason, DROP COLUMN IF EXISTS sandbox;
ALTER TABLE api_keys DROP COLUMN IF EXISTS sandbox, DROP COLUMN IF EXISTS scopes;
