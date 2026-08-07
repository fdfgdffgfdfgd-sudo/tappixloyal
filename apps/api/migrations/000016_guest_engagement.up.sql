ALTER TABLE customers ADD COLUMN IF NOT EXISTS referral_code varchar(20);
UPDATE customers SET referral_code=upper(substr(replace(id::text,'-',''),1,10)) WHERE referral_code IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS customers_referral_code_unique ON customers(referral_code);

CREATE TABLE IF NOT EXISTS customer_wheel_spins (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id),
  customer_id uuid NOT NULL REFERENCES customers(id),
  prize_type varchar(24) NOT NULL,
  prize_value integer NOT NULL DEFAULT 0,
  prize_label varchar(160) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS wheel_customer_created_idx ON customer_wheel_spins(company_id,customer_id,created_at DESC);
ALTER TABLE customer_wheel_spins ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON customer_wheel_spins;
CREATE POLICY tenant_isolation ON customer_wheel_spins USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
