CREATE TABLE IF NOT EXISTS website_settings (
    company_id uuid PRIMARY KEY REFERENCES companies(id),
    headline varchar(200) NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    services jsonb NOT NULL DEFAULT '[]'::jsonb,
    contacts jsonb NOT NULL DEFAULT '{}'::jsonb,
    published boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bookings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    branch_id uuid NOT NULL REFERENCES branches(id),
    customer_name varchar(160) NOT NULL,
    phone varchar(32) NOT NULL,
    service varchar(160) NOT NULL,
    starts_at timestamptz NOT NULL,
    status varchar(24) NOT NULL DEFAULT 'new' CHECK(status IN('new','confirmed','completed','cancelled')),
    comment text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS bookings_company_starts_idx ON bookings(company_id,starts_at);

CREATE TABLE IF NOT EXISTS api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    name varchar(100) NOT NULL,
    prefix varchar(16) NOT NULL,
    secret_hash text NOT NULL UNIQUE,
    last_used_at timestamptz,
    expires_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS api_keys_company_idx ON api_keys(company_id,created_at DESC);

ALTER TABLE website_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE bookings ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON website_settings USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON bookings USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON api_keys USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
