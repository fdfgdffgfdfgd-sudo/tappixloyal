DELETE FROM role_permissions WHERE permission IN ('integrations.read','integrations.manage');
DELETE FROM plan_entitlements WHERE code IN ('integrations','outbound_webhooks');

DROP TABLE IF EXISTS reconciliation_runs;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
DROP TABLE IF EXISTS integration_customer_links;
