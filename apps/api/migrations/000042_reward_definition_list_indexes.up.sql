CREATE INDEX IF NOT EXISTS reward_definitions_company_created_idx
  ON reward_definitions(company_id, created_at DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS reward_rules_active_definition_idx
  ON reward_rules(definition_id)
  WHERE is_active;
