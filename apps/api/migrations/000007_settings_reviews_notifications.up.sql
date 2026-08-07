CREATE TABLE IF NOT EXISTS company_settings (
    company_id uuid PRIMARY KEY REFERENCES companies(id),
    branding jsonb NOT NULL DEFAULT '{}'::jsonb,
    notifications jsonb NOT NULL DEFAULT '{}'::jsonb,
    security jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS review_settings (
    company_id uuid PRIMARY KEY REFERENCES companies(id),
    gis_url text,
    google_url text,
    yandex_url text,
    redirect_threshold numeric(2,1) NOT NULL DEFAULT 4.0,
    enabled boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    customer_id uuid REFERENCES customers(id),
    channel varchar(24) NOT NULL,
    subject varchar(200),
    body text NOT NULL,
    status varchar(24) NOT NULL DEFAULT 'queued',
    sent_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS notifications_company_created_idx ON notifications(company_id,created_at DESC);

ALTER TABLE company_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE review_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON company_settings USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON review_settings USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON notifications USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
