CREATE TABLE IF NOT EXISTS files (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    uploaded_by uuid REFERENCES users(id),
    kind varchar(32) NOT NULL DEFAULT 'asset',
    original_name varchar(255) NOT NULL,
    storage_name varchar(255) NOT NULL UNIQUE,
    content_type varchar(100) NOT NULL,
    size_bytes bigint NOT NULL CHECK(size_bytes > 0 AND size_bytes <= 10485760),
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX IF NOT EXISTS files_company_created_idx ON files(company_id,created_at DESC);

CREATE TABLE IF NOT EXISTS integration_settings (
    company_id uuid PRIMARY KEY REFERENCES companies(id),
    telegram_enabled boolean NOT NULL DEFAULT false,
    sms_enabled boolean NOT NULL DEFAULT false,
    webhook_url text,
    crm_name varchar(100),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE files ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON files USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON integration_settings USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
