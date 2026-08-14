CREATE TABLE operation_risk_flags (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  customer_id uuid REFERENCES customers(id) ON DELETE SET NULL,
  actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
  branch_id uuid REFERENCES branches(id) ON DELETE SET NULL,
  operation varchar(48) NOT NULL,
  severity varchar(16) NOT NULL CHECK(severity IN ('warning','blocked')),
  status varchar(16) NOT NULL DEFAULT 'open' CHECK(status IN ('open','reviewed','dismissed')),
  reason text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  reviewed_at timestamptz,
  reviewed_by uuid REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX operation_risk_flags_company_open_idx ON operation_risk_flags(company_id,created_at DESC) WHERE status='open';
CREATE INDEX operation_risk_flags_customer_idx ON operation_risk_flags(company_id,customer_id,created_at DESC) WHERE customer_id IS NOT NULL;
ALTER TABLE operation_risk_flags ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON operation_risk_flags
  USING (company_id = nullif(current_setting('app.company_id', true), '')::uuid)
  WITH CHECK (company_id = nullif(current_setting('app.company_id', true), '')::uuid);
