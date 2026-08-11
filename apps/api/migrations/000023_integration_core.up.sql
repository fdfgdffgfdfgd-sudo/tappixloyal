CREATE TABLE integration_customer_links (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  connection_id uuid NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
  customer_id uuid REFERENCES customers(id) ON DELETE SET NULL,
  external_customer_id varchar(240) NOT NULL,
  normalized_phone varchar(32),
  status varchar(24) NOT NULL DEFAULT 'linked'
    CHECK(status IN ('pending','linked','conflict','ignored','disabled')),
  match_method varchar(24) NOT NULL DEFAULT 'external_id'
    CHECK(match_method IN ('qr_token','phone','external_id','wallet_barcode','manual')),
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_synced_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(connection_id,external_customer_id)
);
CREATE INDEX integration_customer_links_customer_idx
  ON integration_customer_links(company_id,customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX integration_customer_links_phone_idx
  ON integration_customer_links(company_id,normalized_phone) WHERE normalized_phone IS NOT NULL;
CREATE INDEX integration_customer_links_conflict_idx
  ON integration_customer_links(company_id,connection_id,created_at DESC) WHERE status='conflict';

CREATE TABLE webhook_endpoints (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  connection_id uuid REFERENCES integration_connections(id) ON DELETE CASCADE,
  direction varchar(16) NOT NULL CHECK(direction IN ('inbound','outbound')),
  name varchar(160) NOT NULL,
  url text,
  inbound_key varchar(80),
  encrypted_secret bytea NOT NULL,
  secret_prefix varchar(16) NOT NULL,
  event_types varchar(100)[] NOT NULL DEFAULT '{}',
  api_version varchar(16) NOT NULL DEFAULT '2026-08-01',
  status varchar(24) NOT NULL DEFAULT 'active'
    CHECK(status IN ('active','paused','disabled','error')),
  failure_count integer NOT NULL DEFAULT 0 CHECK(failure_count >= 0),
  last_delivery_at timestamptz,
  last_success_at timestamptz,
  last_error text,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CHECK((direction='outbound' AND url IS NOT NULL AND inbound_key IS NULL) OR
        (direction='inbound' AND url IS NULL AND inbound_key IS NOT NULL))
);
CREATE UNIQUE INDEX webhook_endpoints_inbound_key_uidx
  ON webhook_endpoints(inbound_key) WHERE inbound_key IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX webhook_endpoints_company_idx
  ON webhook_endpoints(company_id,direction,status) WHERE deleted_at IS NULL;

CREATE TABLE webhook_deliveries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  endpoint_id uuid NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
  outbox_event_id uuid REFERENCES outbox_events(id) ON DELETE SET NULL,
  event_type varchar(100) NOT NULL,
  event_id varchar(180) NOT NULL,
  direction varchar(16) NOT NULL CHECK(direction IN ('inbound','outbound')),
  status varchar(24) NOT NULL DEFAULT 'pending'
    CHECK(status IN ('pending','processing','succeeded','failed','dead','rejected')),
  request_headers jsonb NOT NULL DEFAULT '{}'::jsonb,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  response_status integer,
  response_body text,
  attempts integer NOT NULL DEFAULT 0 CHECK(attempts >= 0),
  max_attempts integer NOT NULL DEFAULT 8 CHECK(max_attempts > 0),
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  received_at timestamptz,
  processed_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(endpoint_id,event_id,direction)
);
CREATE INDEX webhook_deliveries_pending_idx
  ON webhook_deliveries(status,next_attempt_at) WHERE status IN ('pending','failed');
CREATE INDEX webhook_deliveries_endpoint_idx
  ON webhook_deliveries(company_id,endpoint_id,created_at DESC);

CREATE TABLE reconciliation_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  connection_id uuid NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
  resource varchar(80) NOT NULL DEFAULT 'transactions',
  status varchar(24) NOT NULL DEFAULT 'pending'
    CHECK(status IN ('pending','processing','succeeded','partial','failed','cancelled')),
  range_start timestamptz NOT NULL,
  range_end timestamptz NOT NULL,
  provider_count integer NOT NULL DEFAULT 0 CHECK(provider_count >= 0),
  local_count integer NOT NULL DEFAULT 0 CHECK(local_count >= 0),
  missing_count integer NOT NULL DEFAULT 0 CHECK(missing_count >= 0),
  mismatch_count integer NOT NULL DEFAULT 0 CHECK(mismatch_count >= 0),
  repaired_count integer NOT NULL DEFAULT 0 CHECK(repaired_count >= 0),
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz,
  completed_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK(range_end > range_start)
);
CREATE INDEX reconciliation_runs_connection_idx
  ON reconciliation_runs(company_id,connection_id,created_at DESC);
CREATE INDEX reconciliation_runs_pending_idx
  ON reconciliation_runs(status,created_at) WHERE status='pending';

ALTER TABLE integration_customer_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_endpoints ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE reconciliation_runs ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON integration_customer_links
  USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid)
  WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON webhook_endpoints
  USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid)
  WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON webhook_deliveries
  USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid)
  WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON reconciliation_runs
  USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid)
  WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);

INSERT INTO plan_entitlements(plan_code,code,enabled,limit_value) VALUES
 ('starter','integrations',false,0),('growth','integrations',true,2),('pro','integrations',true,20),
 ('starter','outbound_webhooks',false,0),('growth','outbound_webhooks',true,2),('pro','outbound_webhooks',true,20)
ON CONFLICT(plan_code,code) DO NOTHING;

INSERT INTO role_permissions(role,permission) VALUES
 ('owner','integrations.read'),('owner','integrations.manage'),
 ('admin','integrations.read'),('admin','integrations.manage'),
 ('manager','integrations.read')
ON CONFLICT DO NOTHING;
