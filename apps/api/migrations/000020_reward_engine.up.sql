CREATE TABLE IF NOT EXISTS reward_definitions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  name varchar(180) NOT NULL,
  description text NOT NULL DEFAULT '',
  reward_type varchar(24) NOT NULL DEFAULT 'gift' CHECK(reward_type IN ('gift','discount','points','service','custom')),
  value integer NOT NULL DEFAULT 0 CHECK(value >= 0),
  validity_days integer CHECK(validity_days IS NULL OR validity_days BETWEEN 1 AND 3650),
  repeatable boolean NOT NULL DEFAULT true,
  cooldown_days integer NOT NULL DEFAULT 0 CHECK(cooldown_days BETWEEN 0 AND 3650),
  inventory_total integer CHECK(inventory_total IS NULL OR inventory_total >= 0),
  inventory_issued integer NOT NULL DEFAULT 0 CHECK(inventory_issued >= 0),
  confirmation_method varchar(24) NOT NULL DEFAULT 'staff' CHECK(confirmation_method IN ('staff','code','qr','automatic')),
  branch_ids uuid[] NOT NULL DEFAULT '{}',
  segment jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE INDEX IF NOT EXISTS reward_definitions_company_idx ON reward_definitions(company_id,is_active) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS reward_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  definition_id uuid NOT NULL REFERENCES reward_definitions(id) ON DELETE CASCADE,
  event_type varchar(40) NOT NULL CHECK(event_type IN ('customer_registered','visit_created','visit_milestone','points_balance','birthday','manual')),
  threshold integer NOT NULL DEFAULT 1 CHECK(threshold > 0),
  progress_mode varchar(24) NOT NULL DEFAULT 'lifetime' CHECK(progress_mode IN ('lifetime','repeat','calendar_month','calendar_year')),
  priority integer NOT NULL DEFAULT 100,
  criteria jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS reward_rules_event_idx ON reward_rules(company_id,event_type,is_active,priority);

CREATE TABLE IF NOT EXISTS customer_reward_progress (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  rule_id uuid NOT NULL REFERENCES reward_rules(id) ON DELETE CASCADE,
  cycle_key varchar(40) NOT NULL DEFAULT 'lifetime',
  current_value integer NOT NULL DEFAULT 0 CHECK(current_value >= 0),
  target_value integer NOT NULL CHECK(target_value > 0),
  status varchar(24) NOT NULL DEFAULT 'locked' CHECK(status IN ('locked','in_progress','available','completed','cancelled')),
  completed_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(customer_id,rule_id,cycle_key)
);
CREATE INDEX IF NOT EXISTS customer_reward_progress_customer_idx ON customer_reward_progress(company_id,customer_id,status);

ALTER TABLE customer_rewards DROP CONSTRAINT IF EXISTS customer_rewards_status_check;
ALTER TABLE customer_rewards
  ADD COLUMN IF NOT EXISTS definition_id uuid REFERENCES reward_definitions(id),
  ADD COLUMN IF NOT EXISTS rule_id uuid REFERENCES reward_rules(id),
  ADD COLUMN IF NOT EXISTS progress_id uuid REFERENCES customer_reward_progress(id),
  ADD COLUMN IF NOT EXISTS reserved_at timestamptz,
  ADD COLUMN IF NOT EXISTS reserved_until timestamptz,
  ADD COLUMN IF NOT EXISTS reserved_by uuid REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS cancelled_at timestamptz,
  ADD COLUMN IF NOT EXISTS cancelled_by uuid REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS idempotency_key varchar(160),
  ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE customer_rewards ADD CONSTRAINT customer_rewards_status_check CHECK(status IN ('locked','in_progress','available','reserved','redeemed','expired','cancelled'));
CREATE UNIQUE INDEX IF NOT EXISTS customer_rewards_idempotency_idx ON customer_rewards(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS customer_rewards_expiry_idx ON customer_rewards(company_id,expires_at) WHERE status IN ('available','reserved');

CREATE TABLE IF NOT EXISTS reward_transactions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  reward_id uuid NOT NULL REFERENCES customer_rewards(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  actor_id uuid REFERENCES users(id),
  operation varchar(24) NOT NULL CHECK(operation IN ('issued','reserved','reservation_released','redeemed','expired','cancelled')),
  from_status varchar(24),
  to_status varchar(24) NOT NULL,
  reason text NOT NULL DEFAULT '',
  idempotency_key varchar(160),
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS reward_transactions_idempotency_idx ON reward_transactions(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS reward_transactions_reward_idx ON reward_transactions(reward_id,created_at);

ALTER TABLE reward_definitions ENABLE ROW LEVEL SECURITY;
ALTER TABLE reward_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_reward_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE reward_transactions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON reward_definitions USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON reward_rules USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON customer_reward_progress USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON reward_transactions USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);

INSERT INTO role_permissions(role,permission) VALUES
 ('owner','rewards.manage'),('admin','rewards.manage'),('manager','rewards.read'),('staff','rewards.read')
ON CONFLICT DO NOTHING;

WITH legacy AS (
  SELECT company_id,coalesce(actions->>'rewardName','Подарок') AS name,coalesce((actions->>'visits')::integer,5) AS threshold
  FROM loyalty_rules WHERE event_type='visit_milestone' AND is_active AND coalesce((actions->>'visits')::integer,0)>0
), created AS (
  INSERT INTO reward_definitions(company_id,name,description,reward_type,validity_days,repeatable,created_at)
  SELECT company_id,name,'Подарок за посещения','gift',90,true,now() FROM legacy
  RETURNING id,company_id,name
)
INSERT INTO reward_rules(company_id,definition_id,event_type,threshold,progress_mode,priority)
SELECT c.company_id,c.id,'visit_created',l.threshold,'repeat',100 FROM created c JOIN legacy l USING(company_id,name);
