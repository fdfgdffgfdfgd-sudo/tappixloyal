ALTER TABLE customers ADD COLUMN IF NOT EXISTS email varchar(254);
CREATE TABLE IF NOT EXISTS marketing_campaigns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id),
    name varchar(160) NOT NULL, channel varchar(20) NOT NULL DEFAULT 'email', subject varchar(200) NOT NULL,
    body text NOT NULL, segment varchar(32) NOT NULL, segment_settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    status varchar(20) NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','sending','sent','partial','failed')),
    audience_count integer NOT NULL DEFAULT 0, sent_count integer NOT NULL DEFAULT 0, failed_count integer NOT NULL DEFAULT 0,
    created_by uuid REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now(), sent_at timestamptz
);
CREATE TABLE IF NOT EXISTS campaign_recipients (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), campaign_id uuid NOT NULL REFERENCES marketing_campaigns(id) ON DELETE CASCADE,
    customer_id uuid NOT NULL REFERENCES customers(id), recipient varchar(254) NOT NULL, status varchar(20) NOT NULL, error text, sent_at timestamptz,
    UNIQUE(campaign_id,customer_id)
);
CREATE INDEX IF NOT EXISTS campaigns_company_created_idx ON marketing_campaigns(company_id,created_at DESC);
ALTER TABLE marketing_campaigns ENABLE ROW LEVEL SECURITY; ALTER TABLE campaign_recipients ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON marketing_campaigns USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON campaign_recipients USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
