DROP INDEX IF EXISTS operation_risk_flags_investigation_idx;
ALTER TABLE operation_risk_flags
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS assigned_to,
  DROP COLUMN IF EXISTS resolution,
  DROP COLUMN IF EXISTS rule_code;
