ALTER TABLE operation_risk_flags
  ADD COLUMN rule_code varchar(64),
  ADD COLUMN resolution text,
  ADD COLUMN assigned_to uuid REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

UPDATE operation_risk_flags SET rule_code=CASE
  WHEN reason ILIKE '%Повтор%' THEN 'duplicate_operation'
  WHEN operation='visit.create' THEN 'rapid_visit'
  WHEN operation LIKE 'bonus.%' THEN 'large_manual_adjustment'
  ELSE 'behavior_anomaly'
END WHERE rule_code IS NULL;

ALTER TABLE operation_risk_flags ALTER COLUMN rule_code SET NOT NULL;
CREATE INDEX operation_risk_flags_investigation_idx
  ON operation_risk_flags(company_id,status,severity,created_at DESC);
