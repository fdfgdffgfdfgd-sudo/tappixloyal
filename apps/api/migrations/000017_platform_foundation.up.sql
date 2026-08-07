CREATE TABLE IF NOT EXISTS plans_v2 (
  code varchar(48) PRIMARY KEY,
  name varchar(120) NOT NULL,
  status varchar(24) NOT NULL DEFAULT 'active' CHECK(status IN('active','archived')),
  monthly_price numeric(12,2) NOT NULL DEFAULT 0,
  currency varchar(3) NOT NULL DEFAULT 'KZT',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS plan_entitlements (
  plan_code varchar(48) NOT NULL REFERENCES plans_v2(code),
  code varchar(64) NOT NULL,
  enabled boolean NOT NULL DEFAULT false,
  limit_value integer,
  PRIMARY KEY(plan_code,code),
  CHECK(limit_value IS NULL OR limit_value >= 0)
);

CREATE TABLE IF NOT EXISTS subscription_overrides (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id),
  entitlement_code varchar(64) NOT NULL,
  enabled boolean,
  limit_value integer,
  reason text NOT NULL,
  valid_until timestamptz,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK(limit_value IS NULL OR limit_value >= 0)
);
CREATE INDEX IF NOT EXISTS subscription_overrides_company_idx ON subscription_overrides(company_id,entitlement_code,valid_until);

CREATE TABLE IF NOT EXISTS support_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id),
  actor_id uuid NOT NULL REFERENCES users(id),
  reason text NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK(expires_at > created_at)
);

CREATE TABLE IF NOT EXISTS loyalty_programs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id),
  name varchar(160) NOT NULL DEFAULT 'Основная программа',
  status varchar(24) NOT NULL DEFAULT 'draft' CHECK(status IN('draft','published','paused','archived')),
  published_version_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS loyalty_programs_one_primary ON loyalty_programs(company_id) WHERE status <> 'archived';

CREATE TABLE IF NOT EXISTS loyalty_program_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id),
  program_id uuid NOT NULL REFERENCES loyalty_programs(id),
  version integer NOT NULL,
  status varchar(24) NOT NULL DEFAULT 'draft' CHECK(status IN('draft','published','archived')),
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by uuid REFERENCES users(id),
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(program_id,version)
);
ALTER TABLE loyalty_programs DROP CONSTRAINT IF EXISTS loyalty_programs_published_version_id_fkey;
ALTER TABLE loyalty_programs ADD CONSTRAINT loyalty_programs_published_version_id_fkey FOREIGN KEY(published_version_id) REFERENCES loyalty_program_versions(id);

CREATE TABLE IF NOT EXISTS smart_links (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id),
  branch_id uuid REFERENCES branches(id),
  device_id uuid REFERENCES devices(id),
  slug varchar(120) NOT NULL,
  source varchar(32) NOT NULL DEFAULT 'direct',
  campaign varchar(120),
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(company_id,slug)
);

CREATE TABLE IF NOT EXISTS smart_link_events (
  id bigserial PRIMARY KEY,
  company_id uuid NOT NULL REFERENCES companies(id),
  smart_link_id uuid NOT NULL REFERENCES smart_links(id),
  event_type varchar(32) NOT NULL,
  source varchar(32), campaign varchar(120), language varchar(10),
  utm_source varchar(120), utm_medium varchar(120), utm_campaign varchar(120),
  request_id varchar(100), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS smart_link_events_company_created_idx ON smart_link_events(company_id,created_at DESC);

CREATE TABLE IF NOT EXISTS outbox_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid REFERENCES companies(id),
  event_type varchar(100) NOT NULL,
  aggregate_type varchar(80) NOT NULL,
  aggregate_id uuid,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  idempotency_key varchar(160) NOT NULL UNIQUE,
  status varchar(24) NOT NULL DEFAULT 'pending' CHECK(status IN('pending','processing','sent','failed','dead')),
  attempts integer NOT NULL DEFAULT 0,
  available_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS outbox_pending_idx ON outbox_events(status,available_at) WHERE status IN('pending','failed');

INSERT INTO plans_v2(code,name,monthly_price) VALUES
 ('starter','Starter',19900),('growth','Growth',49900),('pro','Pro',99900)
ON CONFLICT(code) DO NOTHING;

INSERT INTO plan_entitlements(plan_code,code,enabled,limit_value) VALUES
 ('starter','customers',true,500),('starter','staff',true,2),('starter','smart_links',true,2),('starter','messages_monthly',false,0),
 ('starter','rewards',true,NULL),('starter','whatsapp',false,NULL),('starter','analytics',false,NULL),
 ('growth','customers',true,5000),('growth','staff',true,10),('growth','smart_links',true,20),('growth','messages_monthly',true,2000),
 ('growth','rewards',true,NULL),('growth','whatsapp',true,NULL),('growth','analytics',true,NULL),
 ('pro','customers',true,50000),('pro','staff',true,50),('pro','smart_links',true,200),('pro','messages_monthly',true,20000),
 ('pro','rewards',true,NULL),('pro','whatsapp',true,NULL),('pro','analytics',true,NULL),('pro','api',true,NULL),('pro','custom_domain',true,NULL)
ON CONFLICT(plan_code,code) DO NOTHING;

INSERT INTO loyalty_programs(company_id,status)
SELECT id,'published' FROM companies WHERE deleted_at IS NULL
ON CONFLICT DO NOTHING;

ALTER TABLE subscription_overrides ENABLE ROW LEVEL SECURITY;
ALTER TABLE loyalty_programs ENABLE ROW LEVEL SECURITY;
ALTER TABLE loyalty_program_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE smart_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE smart_link_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON subscription_overrides USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON loyalty_programs USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON loyalty_program_versions USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON smart_links USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON smart_link_events USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON outbox_events USING(company_id IS NULL OR company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id IS NULL OR company_id=nullif(current_setting('app.company_id',true),'')::uuid);
