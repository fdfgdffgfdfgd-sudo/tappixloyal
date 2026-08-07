CREATE TABLE IF NOT EXISTS loyalty_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    customer_id uuid NOT NULL REFERENCES customers(id),
    event_type varchar(64) NOT NULL,
    event_key varchar(120) NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    processed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(company_id,event_key)
);
CREATE INDEX IF NOT EXISTS loyalty_events_company_processed_idx ON loyalty_events(company_id,processed_at DESC);
ALTER TABLE loyalty_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON loyalty_events USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
