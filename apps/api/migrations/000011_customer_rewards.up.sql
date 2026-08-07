CREATE TABLE IF NOT EXISTS customer_rewards (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    customer_id uuid NOT NULL REFERENCES customers(id),
    source_visit_id uuid REFERENCES visits(id),
    name varchar(180) NOT NULL,
    description text NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'available' CHECK(status IN ('available','redeemed','expired','cancelled')),
    issued_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    redeemed_at timestamptz,
    redeemed_by uuid REFERENCES users(id),
    UNIQUE(source_visit_id)
);
CREATE INDEX IF NOT EXISTS customer_rewards_company_customer_idx ON customer_rewards(company_id,customer_id,issued_at DESC);
ALTER TABLE customer_rewards ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON customer_rewards USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
