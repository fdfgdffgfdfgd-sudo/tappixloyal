CREATE TABLE bonus_lots (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  source_ledger_id uuid UNIQUE REFERENCES bonus_ledger(id) ON DELETE SET NULL,
  source_transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL,
  issued_amount integer NOT NULL CHECK(issued_amount > 0),
  remaining_amount integer NOT NULL CHECK(remaining_amount >= 0 AND remaining_amount <= issued_amount),
  monetary_value numeric(14,2) NOT NULL DEFAULT 0 CHECK(monetary_value >= 0),
  issued_at timestamptz NOT NULL DEFAULT now(),
  activates_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz,
  status varchar(20) NOT NULL DEFAULT 'active' CHECK(status IN ('pending','active','redeemed','expired','cancelled')),
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK(expires_at IS NULL OR expires_at > issued_at)
);
CREATE INDEX bonus_lots_fifo_idx ON bonus_lots(company_id,customer_id,activates_at,expires_at,issued_at,id)
  WHERE status IN ('pending','active') AND remaining_amount > 0;
CREATE INDEX bonus_lots_liability_idx ON bonus_lots(company_id,status,expires_at) WHERE remaining_amount > 0;

CREATE TABLE bonus_lot_redemptions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  bonus_lot_id uuid NOT NULL REFERENCES bonus_lots(id) ON DELETE RESTRICT,
  debit_ledger_id uuid NOT NULL REFERENCES bonus_ledger(id) ON DELETE RESTRICT,
  transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL,
  redeemed_amount integer NOT NULL CHECK(redeemed_amount > 0),
  restored_amount integer NOT NULL DEFAULT 0 CHECK(restored_amount >= 0 AND restored_amount <= redeemed_amount),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(bonus_lot_id,debit_ledger_id)
);
CREATE INDEX bonus_lot_redemptions_debit_idx ON bonus_lot_redemptions(company_id,debit_ledger_id);

ALTER TABLE bonus_lots ENABLE ROW LEVEL SECURITY;
ALTER TABLE bonus_lot_redemptions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bonus_lots
  USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid)
  WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON bonus_lot_redemptions
  USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid)
  WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);

-- Preserve the current financial obligation when enabling FIFO for existing tenants.
INSERT INTO bonus_lots(company_id,customer_id,issued_amount,remaining_amount,monetary_value,issued_at,activates_at,status,metadata)
SELECT company_id,id,total_points,total_points,total_points,now(),now(),'active','{"legacyOpeningBalance":true}'::jsonb
FROM customers WHERE total_points > 0 AND deleted_at IS NULL;

INSERT INTO role_permissions(role,permission) VALUES
 ('owner','bonus_liability.read'),('admin','bonus_liability.read'),('manager','bonus_liability.read')
ON CONFLICT DO NOTHING;
