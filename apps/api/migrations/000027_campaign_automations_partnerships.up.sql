CREATE TABLE campaign_automations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  trigger_type varchar(40) NOT NULL CHECK(trigger_type IN ('birthday_bonus','bonus_expiry_3d','winback_30d')),
  name varchar(160) NOT NULL,
  channel varchar(20) NOT NULL DEFAULT 'email' CHECK(channel IN ('email','whatsapp')),
  subject varchar(200) NOT NULL DEFAULT '',
  message text NOT NULL,
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(company_id,trigger_type)
);

CREATE TABLE campaign_automation_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  automation_id uuid NOT NULL REFERENCES campaign_automations(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  trigger_key varchar(180) NOT NULL,
  status varchar(20) NOT NULL CHECK(status IN ('pending','sent','failed','skipped')),
  channel varchar(20) NOT NULL,
  recipient varchar(254),
  error text,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  sent_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(company_id,trigger_key)
);
CREATE INDEX campaign_automation_runs_status_idx ON campaign_automation_runs(company_id,status,created_at DESC);

INSERT INTO campaign_automations(company_id,trigger_type,name,subject,message,settings)
SELECT id,'birthday_bonus','Бонус на день рождения','С днём рождения!','{{name}}, мы начислили вам {{amount}} бонусов. Используйте их при следующем визите.',jsonb_build_object('bonusAmount',500) FROM companies
ON CONFLICT DO NOTHING;
INSERT INTO campaign_automations(company_id,trigger_type,name,subject,message,settings)
SELECT id,'bonus_expiry_3d','Бонусы скоро сгорят','Ваши бонусы сгорят через 3 дня','{{name}}, через 3 дня сгорят {{amount}} бонусов. Успейте воспользоваться ими.',jsonb_build_object('daysBefore',3) FROM companies
ON CONFLICT DO NOTHING;
INSERT INTO campaign_automations(company_id,trigger_type,name,subject,message,settings)
SELECT id,'winback_30d','Мы скучаем','Мы скучаем по вам','{{name}}, вас не было уже 30 дней. Возвращайтесь — будем рады видеть вас снова!',jsonb_build_object('inactiveDays',30) FROM companies
ON CONFLICT DO NOTHING;

CREATE TABLE business_partnerships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  initiator_company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  partner_company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  name varchar(180) NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','active','rejected','paused','ended')),
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  approved_by uuid REFERENCES users(id) ON DELETE SET NULL,
  approved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK(initiator_company_id<>partner_company_id)
);
CREATE UNIQUE INDEX business_partnerships_pair_active_idx ON business_partnerships(least(initiator_company_id,partner_company_id),greatest(initiator_company_id,partner_company_id)) WHERE status IN ('pending','active','paused');

CREATE TABLE partnership_offers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  partnership_id uuid NOT NULL REFERENCES business_partnerships(id) ON DELETE CASCADE,
  source_company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  reward_company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  code varchar(40) NOT NULL,
  name varchar(180) NOT NULL,
  reward_points integer NOT NULL CHECK(reward_points>0),
  minimum_source_purchase numeric(16,2) NOT NULL DEFAULT 0 CHECK(minimum_source_purchase>=0),
  max_redemptions integer CHECK(max_redemptions IS NULL OR max_redemptions>0),
  max_per_customer integer NOT NULL DEFAULT 1 CHECK(max_per_customer BETWEEN 1 AND 100),
  starts_at timestamptz NOT NULL DEFAULT now(),
  ends_at timestamptz,
  is_active boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK(source_company_id<>reward_company_id),
  CHECK(ends_at IS NULL OR ends_at>starts_at)
);
CREATE UNIQUE INDEX partnership_offers_code_uidx ON partnership_offers(lower(code));

CREATE TABLE partnership_redemptions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  partnership_id uuid NOT NULL REFERENCES business_partnerships(id) ON DELETE CASCADE,
  offer_id uuid NOT NULL REFERENCES partnership_offers(id) ON DELETE CASCADE,
  source_company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  reward_company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  target_customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
  source_transaction_id uuid NOT NULL REFERENCES sales_transactions(id) ON DELETE RESTRICT,
  bonus_ledger_id uuid REFERENCES bonus_ledger(id) ON DELETE SET NULL,
  reward_points integer NOT NULL CHECK(reward_points>0),
  status varchar(20) NOT NULL DEFAULT 'rewarded' CHECK(status IN ('rewarded','reversed','review')),
  idempotency_key varchar(180) NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  redeemed_at timestamptz NOT NULL DEFAULT now(),
  reversed_at timestamptz,
  UNIQUE(source_company_id,idempotency_key),
  UNIQUE(offer_id,source_transaction_id,target_customer_id)
);
CREATE INDEX partnership_redemptions_offer_idx ON partnership_redemptions(offer_id,redeemed_at DESC);

ALTER TABLE campaign_automations ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_automation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE business_partnerships ENABLE ROW LEVEL SECURITY;
ALTER TABLE partnership_offers ENABLE ROW LEVEL SECURITY;
ALTER TABLE partnership_redemptions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaign_automations USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY tenant_isolation ON campaign_automation_runs USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY partnership_member_access ON business_partnerships USING(initiator_company_id=nullif(current_setting('app.company_id',true),'')::uuid OR partner_company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(initiator_company_id=nullif(current_setting('app.company_id',true),'')::uuid OR partner_company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY partnership_offer_access ON partnership_offers USING(source_company_id=nullif(current_setting('app.company_id',true),'')::uuid OR reward_company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(source_company_id=nullif(current_setting('app.company_id',true),'')::uuid OR reward_company_id=nullif(current_setting('app.company_id',true),'')::uuid);
CREATE POLICY partnership_redemption_access ON partnership_redemptions USING(source_company_id=nullif(current_setting('app.company_id',true),'')::uuid OR reward_company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(source_company_id=nullif(current_setting('app.company_id',true),'')::uuid OR reward_company_id=nullif(current_setting('app.company_id',true),'')::uuid);

INSERT INTO modules(code,name,description) VALUES('partnerships','Партнёрства','Совместные промокоды и кросс-маркетинг') ON CONFLICT(code) DO NOTHING;
INSERT INTO plan_entitlements(plan_code,code,enabled,limit_value) VALUES('starter','partnerships',false,0),('growth','partnerships',true,5),('pro','partnerships',true,100) ON CONFLICT(plan_code,code) DO NOTHING;
INSERT INTO role_permissions(role,permission) VALUES
 ('owner','automations.manage'),('admin','automations.manage'),('owner','automations.read'),('admin','automations.read'),('manager','automations.read'),
 ('owner','partnerships.manage'),('admin','partnerships.manage'),('owner','partnerships.read'),('admin','partnerships.read'),('manager','partnerships.read') ON CONFLICT DO NOTHING;
