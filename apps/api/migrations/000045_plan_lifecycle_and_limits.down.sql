DROP INDEX IF EXISTS integration_jobs_connection_created_idx;
DROP INDEX IF EXISTS subscriptions_company_created_idx;
DELETE FROM plan_entitlements WHERE code IN ('branches','integrations','export');
