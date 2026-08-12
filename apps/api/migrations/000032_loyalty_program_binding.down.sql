DROP INDEX IF EXISTS company_settings_loyalty_rule_uidx;
DROP INDEX IF EXISTS company_settings_loyalty_definition_uidx;
ALTER TABLE company_settings DROP COLUMN IF EXISTS loyalty_reward_rule_id, DROP COLUMN IF EXISTS loyalty_reward_definition_id;
