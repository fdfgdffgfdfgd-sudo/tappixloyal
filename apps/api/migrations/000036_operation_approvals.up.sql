CREATE TABLE operation_approvals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  requested_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  branch_id uuid NOT NULL REFERENCES branches(id) ON DELETE RESTRICT,
  operation varchar(32) NOT NULL CHECK(operation IN ('bonus.credit','bonus.debit')),
  amount integer NOT NULL CHECK(amount > 0),
  reason text NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected','expired','cancelled')),
  idempotency_key varchar(180),
  requested_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL DEFAULT now()+interval '24 hours',
  decided_at timestamptz,
  decided_by uuid REFERENCES users(id) ON DELETE SET NULL,
  decision_reason text,
  executed_at timestamptz,
  bonus_ledger_id uuid REFERENCES bonus_ledger(id) ON DELETE SET NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE(company_id,idempotency_key)
);
CREATE INDEX operation_approvals_company_status_idx ON operation_approvals(company_id,status,requested_at DESC);
CREATE INDEX operation_approvals_customer_idx ON operation_approvals(company_id,customer_id,requested_at DESC);
ALTER TABLE operation_approvals ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON operation_approvals
  USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid)
  WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
