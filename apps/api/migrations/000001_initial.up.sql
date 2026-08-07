CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE company_status AS ENUM ('active', 'blocked', 'archived');
CREATE TYPE user_role AS ENUM ('super_admin', 'company_owner', 'employee');
CREATE TYPE user_status AS ENUM ('invited', 'active', 'blocked');
CREATE TYPE bonus_operation AS ENUM ('credit', 'debit', 'expire', 'adjustment');
CREATE TYPE subscription_status AS ENUM ('trial', 'active', 'past_due', 'cancelled', 'expired');
CREATE TYPE billing_period AS ENUM ('monthly', 'quarterly', 'yearly', 'custom');

CREATE TABLE companies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(160) NOT NULL,
    slug varchar(80) NOT NULL UNIQUE,
    logo_url text,
    phone varchar(32),
    email varchar(254),
    address text,
    timezone varchar(64) NOT NULL DEFAULT 'Asia/Almaty',
    language varchar(10) NOT NULL DEFAULT 'ru',
    status company_status NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TABLE branches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    name varchar(160) NOT NULL,
    phone varchar(32),
    address text NOT NULL,
    latitude numeric(9,6),
    longitude numeric(9,6),
    working_hours jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (company_id, name)
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid REFERENCES companies(id),
    branch_id uuid REFERENCES branches(id),
    first_name varchar(80) NOT NULL,
    last_name varchar(80) NOT NULL DEFAULT '',
    email varchar(254) NOT NULL,
    password_hash text NOT NULL,
    role user_role NOT NULL,
    status user_status NOT NULL DEFAULT 'active',
    last_login_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT super_admin_without_company CHECK (
      (role = 'super_admin' AND company_id IS NULL) OR
      (role <> 'super_admin' AND company_id IS NOT NULL)
    )
);
CREATE UNIQUE INDEX users_email_unique_active ON users (lower(email)) WHERE deleted_at IS NULL;

CREATE TABLE customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    first_name varchar(80) NOT NULL,
    last_name varchar(80) NOT NULL DEFAULT '',
    phone varchar(32) NOT NULL,
    birthday date,
    gender varchar(24),
    total_points integer NOT NULL DEFAULT 0 CHECK (total_points >= 0),
    total_visits integer NOT NULL DEFAULT 0 CHECK (total_visits >= 0),
    level varchar(48) NOT NULL DEFAULT 'basic',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE UNIQUE INDEX customers_company_phone_unique_active ON customers (company_id, phone) WHERE deleted_at IS NULL;
CREATE INDEX customers_company_name_idx ON customers (company_id, lower(last_name), lower(first_name));
CREATE INDEX customers_company_created_idx ON customers (company_id, created_at DESC);

CREATE TABLE loyalty_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    name varchar(160) NOT NULL,
    event_type varchar(64) NOT NULL,
    conditions jsonb NOT NULL DEFAULT '{}'::jsonb,
    actions jsonb NOT NULL DEFAULT '[]'::jsonb,
    priority integer NOT NULL DEFAULT 100,
    is_active boolean NOT NULL DEFAULT true,
    starts_at timestamptz,
    ends_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at)
);
CREATE INDEX loyalty_rules_company_event_idx ON loyalty_rules (company_id, event_type) WHERE is_active;

CREATE TABLE visits (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    branch_id uuid NOT NULL REFERENCES branches(id),
    customer_id uuid NOT NULL REFERENCES customers(id),
    employee_id uuid REFERENCES users(id),
    points_added integer NOT NULL DEFAULT 0 CHECK (points_added >= 0),
    comment text,
    idempotency_key varchar(120),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, idempotency_key)
);
CREATE INDEX visits_customer_created_idx ON visits (company_id, customer_id, created_at DESC);
CREATE INDEX visits_branch_created_idx ON visits (company_id, branch_id, created_at DESC);

CREATE TABLE bonus_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    customer_id uuid NOT NULL REFERENCES customers(id),
    visit_id uuid REFERENCES visits(id),
    created_by uuid REFERENCES users(id),
    operation bonus_operation NOT NULL,
    amount integer NOT NULL CHECK (amount > 0),
    balance_after integer NOT NULL CHECK (balance_after >= 0),
    description text NOT NULL,
    idempotency_key varchar(120),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, idempotency_key)
);
CREATE INDEX bonus_ledger_customer_created_idx ON bonus_ledger (company_id, customer_id, created_at DESC);

CREATE TABLE modules (
    code varchar(48) PRIMARY KEY,
    name varchar(100) NOT NULL,
    description text,
    is_core boolean NOT NULL DEFAULT false
);

INSERT INTO modules (code, name, is_core) VALUES
 ('core', 'Core', true), ('crm', 'CRM', false), ('loyalty', 'Loyalty', false),
 ('analytics', 'Analytics', false), ('website', 'Website', false), ('reviews', 'Reviews', false),
 ('telegram', 'Telegram', false), ('email', 'Email', false), ('sms', 'SMS', false),
 ('api', 'API', false), ('booking', 'Booking', false);

CREATE TABLE company_modules (
    company_id uuid NOT NULL REFERENCES companies(id),
    module_code varchar(48) NOT NULL REFERENCES modules(code),
    enabled boolean NOT NULL DEFAULT false,
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, module_code)
);

CREATE TABLE subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    plan_code varchar(64) NOT NULL,
    status subscription_status NOT NULL DEFAULT 'trial',
    amount numeric(12,2) NOT NULL DEFAULT 0 CHECK (amount >= 0),
    currency char(3) NOT NULL DEFAULT 'KZT',
    billing_period billing_period NOT NULL DEFAULT 'monthly',
    starts_at timestamptz NOT NULL DEFAULT now(),
    current_period_ends_at timestamptz,
    cancelled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX subscriptions_one_current_idx ON subscriptions (company_id) WHERE status IN ('trial', 'active', 'past_due');

CREATE TABLE devices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    branch_id uuid NOT NULL REFERENCES branches(id),
    kind varchar(12) NOT NULL CHECK (kind IN ('nfc', 'qr')),
    name varchar(100) NOT NULL,
    token varchar(100) NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(24), 'hex'),
    destination varchar(32) NOT NULL DEFAULT 'join',
    is_active boolean NOT NULL DEFAULT true,
    scans_count bigint NOT NULL DEFAULT 0,
    last_scanned_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX devices_company_branch_idx ON devices (company_id, branch_id);

CREATE TABLE audit_logs (
    id bigserial PRIMARY KEY,
    company_id uuid REFERENCES companies(id),
    actor_id uuid REFERENCES users(id),
    action varchar(100) NOT NULL,
    entity_type varchar(80) NOT NULL,
    entity_id uuid,
    request_id varchar(100),
    ip inet,
    user_agent text,
    before_data jsonb,
    after_data jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_company_created_idx ON audit_logs (company_id, created_at DESC);

-- Every tenant table is protected by PostgreSQL RLS as defense in depth.
ALTER TABLE branches ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers ENABLE ROW LEVEL SECURITY;
ALTER TABLE loyalty_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE visits ENABLE ROW LEVEL SECURITY;
ALTER TABLE bonus_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE company_modules ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

DO $$
DECLARE table_name text;
BEGIN
  FOREACH table_name IN ARRAY ARRAY['branches','users','customers','loyalty_rules','visits','bonus_ledger','company_modules','subscriptions','devices','audit_logs']
  LOOP
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (company_id = nullif(current_setting(''app.company_id'', true), '''')::uuid) WITH CHECK (company_id = nullif(current_setting(''app.company_id'', true), '''')::uuid)', table_name);
  END LOOP;
END $$;
