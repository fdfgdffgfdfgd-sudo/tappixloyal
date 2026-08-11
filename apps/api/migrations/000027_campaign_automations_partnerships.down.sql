DELETE FROM role_permissions WHERE permission IN ('automations.manage','automations.read','partnerships.manage','partnerships.read');
DELETE FROM plan_entitlements WHERE code='partnerships';
DELETE FROM modules WHERE code='partnerships';
DROP TABLE IF EXISTS partnership_redemptions;
DROP TABLE IF EXISTS partnership_offers;
DROP TABLE IF EXISTS business_partnerships;
DROP TABLE IF EXISTS campaign_automation_runs;
DROP TABLE IF EXISTS campaign_automations;
