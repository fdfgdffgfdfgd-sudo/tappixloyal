ALTER TABLE company_settings
  ADD COLUMN loyalty_reward_definition_id uuid REFERENCES reward_definitions(id) ON DELETE SET NULL,
  ADD COLUMN loyalty_reward_rule_id uuid REFERENCES reward_rules(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX company_settings_loyalty_definition_uidx ON company_settings(loyalty_reward_definition_id) WHERE loyalty_reward_definition_id IS NOT NULL;
CREATE UNIQUE INDEX company_settings_loyalty_rule_uidx ON company_settings(loyalty_reward_rule_id) WHERE loyalty_reward_rule_id IS NOT NULL;
