-- Canonical business data layer for POS transactions, customer journeys,
-- campaign attribution, referrals, analytics projections and scheduled reports.

CREATE TABLE integration_connections (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  provider varchar(48) NOT NULL,
  name varchar(160) NOT NULL,
  status varchar(24) NOT NULL DEFAULT 'draft'
    CHECK(status IN ('draft','connecting','active','degraded','error','disabled')),
  auth_type varchar(24) NOT NULL DEFAULT 'api_key'
    CHECK(auth_type IN ('api_key','oauth2','basic','certificate','local_agent','none')),
  encrypted_credentials bytea,
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  capabilities jsonb NOT NULL DEFAULT '{}'::jsonb,
  external_account_id varchar(200),
  last_connected_at timestamptz,
  last_sync_at timestamptz,
  last_error_code varchar(80),
  last_error_message text,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX integration_connections_provider_account_uidx
  ON integration_connections(company_id,provider,external_account_id)
  WHERE external_account_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX integration_connections_company_status_idx
  ON integration_connections(company_id,status) WHERE deleted_at IS NULL;

CREATE TABLE integration_location_mappings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  connection_id uuid NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
  branch_id uuid REFERENCES branches(id) ON DELETE SET NULL,
  external_location_id varchar(200) NOT NULL,
  external_location_name varchar(240) NOT NULL DEFAULT '',
  status varchar(24) NOT NULL DEFAULT 'mapped'
    CHECK(status IN ('unmapped','mapped','ignored','disabled')),
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(connection_id,external_location_id)
);
CREATE INDEX integration_location_mappings_branch_idx
  ON integration_location_mappings(company_id,branch_id) WHERE branch_id IS NOT NULL;

CREATE TABLE sales_transactions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  branch_id uuid REFERENCES branches(id) ON DELETE SET NULL,
  customer_id uuid REFERENCES customers(id) ON DELETE SET NULL,
  integration_connection_id uuid REFERENCES integration_connections(id) ON DELETE SET NULL,
  provider varchar(48) NOT NULL,
  external_id varchar(240) NOT NULL,
  status varchar(24) NOT NULL DEFAULT 'completed'
    CHECK(status IN ('pending','completed','partially_refunded','refunded','cancelled')),
  occurred_at timestamptz NOT NULL,
  gross_amount numeric(16,2) NOT NULL CHECK(gross_amount >= 0),
  discount_amount numeric(16,2) NOT NULL DEFAULT 0 CHECK(discount_amount >= 0),
  bonus_paid_amount numeric(16,2) NOT NULL DEFAULT 0 CHECK(bonus_paid_amount >= 0),
  cash_paid_amount numeric(16,2) NOT NULL DEFAULT 0 CHECK(cash_paid_amount >= 0),
  net_amount numeric(16,2) NOT NULL CHECK(net_amount >= 0),
  cost_amount numeric(16,2) CHECK(cost_amount IS NULL OR cost_amount >= 0),
  currency varchar(3) NOT NULL DEFAULT 'KZT' CHECK(currency = upper(currency)),
  receipt_number varchar(120),
  employee_id uuid REFERENCES users(id) ON DELETE SET NULL,
  source varchar(48) NOT NULL DEFAULT 'pos',
  campaign_id uuid REFERENCES marketing_campaigns(id) ON DELETE SET NULL,
  original_transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL,
  idempotency_key varchar(180),
  raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(company_id,provider,external_id),
  CHECK(original_transaction_id IS NULL OR original_transaction_id <> id)
);
CREATE UNIQUE INDEX sales_transactions_idempotency_uidx
  ON sales_transactions(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX sales_transactions_company_occurred_idx
  ON sales_transactions(company_id,occurred_at DESC);
CREATE INDEX sales_transactions_customer_occurred_idx
  ON sales_transactions(company_id,customer_id,occurred_at DESC) WHERE customer_id IS NOT NULL;
CREATE INDEX sales_transactions_branch_occurred_idx
  ON sales_transactions(company_id,branch_id,occurred_at DESC) WHERE branch_id IS NOT NULL;
CREATE INDEX sales_transactions_campaign_idx
  ON sales_transactions(company_id,campaign_id,occurred_at DESC) WHERE campaign_id IS NOT NULL;

CREATE TABLE sales_transaction_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  transaction_id uuid NOT NULL REFERENCES sales_transactions(id) ON DELETE CASCADE,
  external_item_id varchar(240),
  product_external_id varchar(240),
  sku varchar(160),
  name varchar(300) NOT NULL,
  category varchar(200),
  quantity numeric(14,3) NOT NULL CHECK(quantity > 0),
  unit_price numeric(16,2) NOT NULL CHECK(unit_price >= 0),
  gross_amount numeric(16,2) NOT NULL CHECK(gross_amount >= 0),
  discount_amount numeric(16,2) NOT NULL DEFAULT 0 CHECK(discount_amount >= 0),
  net_amount numeric(16,2) NOT NULL CHECK(net_amount >= 0),
  cost_amount numeric(16,2) CHECK(cost_amount IS NULL OR cost_amount >= 0),
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sales_transaction_items_transaction_idx
  ON sales_transaction_items(company_id,transaction_id);
CREATE INDEX sales_transaction_items_product_idx
  ON sales_transaction_items(company_id,product_external_id) WHERE product_external_id IS NOT NULL;

CREATE TABLE payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  transaction_id uuid NOT NULL REFERENCES sales_transactions(id) ON DELETE CASCADE,
  provider varchar(48),
  external_id varchar(240),
  payment_type varchar(32) NOT NULL
    CHECK(payment_type IN ('cash','card','bank_transfer','qr','wallet','bonus','gift_card','mixed','other')),
  status varchar(24) NOT NULL DEFAULT 'captured'
    CHECK(status IN ('pending','authorized','captured','partially_refunded','refunded','voided','failed')),
  amount numeric(16,2) NOT NULL CHECK(amount >= 0),
  currency varchar(3) NOT NULL DEFAULT 'KZT' CHECK(currency = upper(currency)),
  occurred_at timestamptz NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX payments_external_uidx
  ON payments(company_id,provider,external_id)
  WHERE provider IS NOT NULL AND external_id IS NOT NULL;
CREATE INDEX payments_transaction_idx ON payments(company_id,transaction_id);

CREATE TABLE customer_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  customer_id uuid REFERENCES customers(id) ON DELETE SET NULL,
  anonymous_id varchar(160),
  event_type varchar(80) NOT NULL,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  branch_id uuid REFERENCES branches(id) ON DELETE SET NULL,
  device_id uuid REFERENCES devices(id) ON DELETE SET NULL,
  smart_link_id uuid REFERENCES smart_links(id) ON DELETE SET NULL,
  transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL,
  campaign_id uuid REFERENCES marketing_campaigns(id) ON DELETE SET NULL,
  referral_code varchar(40),
  session_id varchar(160),
  source varchar(48),
  properties jsonb NOT NULL DEFAULT '{}'::jsonb,
  idempotency_key varchar(180),
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK(customer_id IS NOT NULL OR anonymous_id IS NOT NULL)
);
CREATE UNIQUE INDEX customer_events_idempotency_uidx
  ON customer_events(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX customer_events_customer_time_idx
  ON customer_events(company_id,customer_id,occurred_at DESC) WHERE customer_id IS NOT NULL;
CREATE INDEX customer_events_type_time_idx
  ON customer_events(company_id,event_type,occurred_at DESC);
CREATE INDEX customer_events_anonymous_idx
  ON customer_events(company_id,anonymous_id,occurred_at DESC) WHERE anonymous_id IS NOT NULL;

CREATE TABLE customer_attributions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  attribution_type varchar(24) NOT NULL DEFAULT 'first_touch'
    CHECK(attribution_type IN ('first_touch','last_touch','registration','transaction')),
  source varchar(80) NOT NULL,
  medium varchar(80),
  campaign varchar(160),
  content varchar(160),
  term varchar(160),
  referrer text,
  smart_link_id uuid REFERENCES smart_links(id) ON DELETE SET NULL,
  device_id uuid REFERENCES devices(id) ON DELETE SET NULL,
  campaign_id uuid REFERENCES marketing_campaigns(id) ON DELETE SET NULL,
  transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL,
  attributed_at timestamptz NOT NULL DEFAULT now(),
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX customer_attributions_customer_idx
  ON customer_attributions(company_id,customer_id,attributed_at DESC);
CREATE INDEX customer_attributions_campaign_idx
  ON customer_attributions(company_id,campaign_id,attributed_at DESC) WHERE campaign_id IS NOT NULL;

CREATE TABLE integration_sync_cursors (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  connection_id uuid NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
  resource varchar(80) NOT NULL,
  cursor_value text,
  watermark_at timestamptz,
  last_success_at timestamptz,
  last_attempt_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(connection_id,resource)
);

CREATE TABLE integration_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  connection_id uuid NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
  job_type varchar(64) NOT NULL,
  resource varchar(80),
  status varchar(24) NOT NULL DEFAULT 'pending'
    CHECK(status IN ('pending','processing','succeeded','failed','dead','cancelled')),
  idempotency_key varchar(180),
  cursor_value text,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  result jsonb NOT NULL DEFAULT '{}'::jsonb,
  attempts integer NOT NULL DEFAULT 0 CHECK(attempts >= 0),
  max_attempts integer NOT NULL DEFAULT 8 CHECK(max_attempts > 0),
  available_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  completed_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX integration_jobs_idempotency_uidx
  ON integration_jobs(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX integration_jobs_pending_idx
  ON integration_jobs(status,available_at) WHERE status IN ('pending','failed');
CREATE INDEX integration_jobs_connection_idx
  ON integration_jobs(company_id,connection_id,created_at DESC);

CREATE TABLE integration_failures (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  connection_id uuid NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
  job_id uuid REFERENCES integration_jobs(id) ON DELETE SET NULL,
  resource varchar(80),
  external_id varchar(240),
  error_code varchar(100) NOT NULL,
  error_message text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  retryable boolean NOT NULL DEFAULT true,
  resolved_at timestamptz,
  resolved_by uuid REFERENCES users(id) ON DELETE SET NULL,
  resolution_note text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX integration_failures_open_idx
  ON integration_failures(company_id,connection_id,created_at DESC) WHERE resolved_at IS NULL;

CREATE TABLE campaign_conversions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  campaign_id uuid NOT NULL REFERENCES marketing_campaigns(id) ON DELETE CASCADE,
  campaign_recipient_id uuid REFERENCES campaign_recipients(id) ON DELETE SET NULL,
  customer_id uuid REFERENCES customers(id) ON DELETE SET NULL,
  transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL,
  conversion_type varchar(40) NOT NULL
    CHECK(conversion_type IN ('delivered','opened','clicked','registered','purchased','redeemed')),
  conversion_value numeric(16,2) NOT NULL DEFAULT 0 CHECK(conversion_value >= 0),
  currency varchar(3) NOT NULL DEFAULT 'KZT' CHECK(currency = upper(currency)),
  attribution_model varchar(24) NOT NULL DEFAULT 'last_touch'
    CHECK(attribution_model IN ('first_touch','last_touch','linear','holdout')),
  occurred_at timestamptz NOT NULL DEFAULT now(),
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  idempotency_key varchar(180),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX campaign_conversions_idempotency_uidx
  ON campaign_conversions(company_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX campaign_conversions_campaign_time_idx
  ON campaign_conversions(company_id,campaign_id,occurred_at DESC);

CREATE TABLE referral_programs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  name varchar(160) NOT NULL DEFAULT 'Реферальная программа',
  status varchar(24) NOT NULL DEFAULT 'draft'
    CHECK(status IN ('draft','active','paused','archived')),
  referrer_reward_type varchar(24) NOT NULL DEFAULT 'points'
    CHECK(referrer_reward_type IN ('points','fixed','percent','reward')),
  referrer_reward_value numeric(16,2) NOT NULL DEFAULT 0 CHECK(referrer_reward_value >= 0),
  friend_reward_type varchar(24) NOT NULL DEFAULT 'points'
    CHECK(friend_reward_type IN ('points','fixed','percent','reward')),
  friend_reward_value numeric(16,2) NOT NULL DEFAULT 0 CHECK(friend_reward_value >= 0),
  qualification_event varchar(40) NOT NULL DEFAULT 'first_paid_purchase'
    CHECK(qualification_event IN ('registration','first_paid_purchase','purchase_threshold')),
  minimum_purchase_amount numeric(16,2) NOT NULL DEFAULT 0 CHECK(minimum_purchase_amount >= 0),
  reward_delay_days integer NOT NULL DEFAULT 0 CHECK(reward_delay_days BETWEEN 0 AND 365),
  max_rewards_per_customer integer CHECK(max_rewards_per_customer IS NULL OR max_rewards_per_customer > 0),
  max_rewards_per_month integer CHECK(max_rewards_per_month IS NULL OR max_rewards_per_month > 0),
  starts_at timestamptz,
  ends_at timestamptz,
  rules jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK(ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at)
);
CREATE UNIQUE INDEX referral_programs_one_active_uidx
  ON referral_programs(company_id) WHERE status = 'active';

CREATE TABLE referral_attributions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  program_id uuid NOT NULL REFERENCES referral_programs(id) ON DELETE CASCADE,
  referrer_customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  referred_customer_id uuid REFERENCES customers(id) ON DELETE SET NULL,
  referral_code varchar(40) NOT NULL,
  anonymous_id varchar(160),
  source varchar(80) NOT NULL DEFAULT 'direct',
  status varchar(24) NOT NULL DEFAULT 'clicked'
    CHECK(status IN ('clicked','registered','qualified','reward_pending','rewarded','rejected','reversed')),
  clicked_at timestamptz NOT NULL DEFAULT now(),
  registered_at timestamptz,
  qualified_at timestamptz,
  qualifying_transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL,
  rejection_reason text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK(referred_customer_id IS NULL OR referred_customer_id <> referrer_customer_id)
);
CREATE UNIQUE INDEX referral_attributions_referred_uidx
  ON referral_attributions(company_id,program_id,referred_customer_id)
  WHERE referred_customer_id IS NOT NULL AND status <> 'rejected';
CREATE INDEX referral_attributions_referrer_idx
  ON referral_attributions(company_id,referrer_customer_id,created_at DESC);
CREATE INDEX referral_attributions_code_idx
  ON referral_attributions(company_id,referral_code,created_at DESC);

CREATE TABLE referral_rewards (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  attribution_id uuid NOT NULL REFERENCES referral_attributions(id) ON DELETE CASCADE,
  beneficiary_customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  beneficiary_type varchar(16) NOT NULL CHECK(beneficiary_type IN ('referrer','friend')),
  reward_type varchar(24) NOT NULL CHECK(reward_type IN ('points','fixed','percent','reward')),
  reward_value numeric(16,2) NOT NULL CHECK(reward_value >= 0),
  status varchar(24) NOT NULL DEFAULT 'pending'
    CHECK(status IN ('pending','issued','rejected','reversed','expired')),
  available_at timestamptz NOT NULL DEFAULT now(),
  issued_at timestamptz,
  reversed_at timestamptz,
  bonus_ledger_id uuid REFERENCES bonus_ledger(id) ON DELETE SET NULL,
  customer_reward_id uuid REFERENCES customer_rewards(id) ON DELETE SET NULL,
  idempotency_key varchar(180) NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(company_id,idempotency_key),
  UNIQUE(attribution_id,beneficiary_type)
);
CREATE INDEX referral_rewards_pending_idx
  ON referral_rewards(company_id,status,available_at) WHERE status = 'pending';

CREATE TABLE analytics_daily_facts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  fact_date date NOT NULL,
  branch_id uuid REFERENCES branches(id) ON DELETE CASCADE,
  currency varchar(3) NOT NULL DEFAULT 'KZT' CHECK(currency = upper(currency)),
  registrations integer NOT NULL DEFAULT 0 CHECK(registrations >= 0),
  active_customers integer NOT NULL DEFAULT 0 CHECK(active_customers >= 0),
  new_buyers integer NOT NULL DEFAULT 0 CHECK(new_buyers >= 0),
  repeat_buyers integer NOT NULL DEFAULT 0 CHECK(repeat_buyers >= 0),
  completed_transactions integer NOT NULL DEFAULT 0 CHECK(completed_transactions >= 0),
  refunded_transactions integer NOT NULL DEFAULT 0 CHECK(refunded_transactions >= 0),
  gross_revenue numeric(18,2) NOT NULL DEFAULT 0,
  net_revenue numeric(18,2) NOT NULL DEFAULT 0,
  discount_amount numeric(18,2) NOT NULL DEFAULT 0,
  bonus_issued_value numeric(18,2) NOT NULL DEFAULT 0,
  bonus_redeemed_value numeric(18,2) NOT NULL DEFAULT 0,
  bonus_expired_value numeric(18,2) NOT NULL DEFAULT 0,
  campaign_cost numeric(18,2) NOT NULL DEFAULT 0,
  attributed_revenue numeric(18,2) NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX analytics_daily_facts_company_branch_date_uidx
  ON analytics_daily_facts(company_id,fact_date,coalesce(branch_id,'00000000-0000-0000-0000-000000000000'::uuid));
CREATE INDEX analytics_daily_facts_date_idx
  ON analytics_daily_facts(company_id,fact_date DESC);

CREATE TABLE analytics_customer_features (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  calculated_at timestamptz NOT NULL DEFAULT now(),
  first_purchase_at timestamptz,
  last_purchase_at timestamptz,
  purchase_count integer NOT NULL DEFAULT 0 CHECK(purchase_count >= 0),
  lifetime_revenue numeric(18,2) NOT NULL DEFAULT 0,
  average_check numeric(18,2) NOT NULL DEFAULT 0,
  days_since_last_purchase integer CHECK(days_since_last_purchase IS NULL OR days_since_last_purchase >= 0),
  median_purchase_interval_days numeric(10,2),
  recency_score smallint CHECK(recency_score BETWEEN 1 AND 5),
  frequency_score smallint CHECK(frequency_score BETWEEN 1 AND 5),
  monetary_score smallint CHECK(monetary_score BETWEEN 1 AND 5),
  rfm_segment varchar(48),
  churn_risk varchar(16) CHECK(churn_risk IN ('unknown','low','medium','high')),
  predicted_ltv numeric(18,2),
  referral_count integer NOT NULL DEFAULT 0 CHECK(referral_count >= 0),
  referral_revenue numeric(18,2) NOT NULL DEFAULT 0,
  features jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE(company_id,customer_id)
);
CREATE INDEX analytics_customer_features_rfm_idx
  ON analytics_customer_features(company_id,rfm_segment,churn_risk);
CREATE INDEX analytics_customer_features_value_idx
  ON analytics_customer_features(company_id,lifetime_revenue DESC);

CREATE TABLE report_schedules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  name varchar(160) NOT NULL,
  report_type varchar(48) NOT NULL,
  channel varchar(24) NOT NULL CHECK(channel IN ('email','whatsapp','webhook')),
  recipients jsonb NOT NULL DEFAULT '[]'::jsonb,
  frequency varchar(24) NOT NULL CHECK(frequency IN ('daily','weekly','monthly')),
  timezone varchar(64) NOT NULL DEFAULT 'Asia/Almaty',
  send_hour smallint NOT NULL DEFAULT 9 CHECK(send_hour BETWEEN 0 AND 23),
  send_weekday smallint CHECK(send_weekday BETWEEN 1 AND 7),
  send_monthday smallint CHECK(send_monthday BETWEEN 1 AND 28),
  filters jsonb NOT NULL DEFAULT '{}'::jsonb,
  format varchar(16) NOT NULL DEFAULT 'summary' CHECK(format IN ('summary','csv','xlsx','pdf')),
  is_active boolean NOT NULL DEFAULT true,
  next_run_at timestamptz,
  last_run_at timestamptz,
  last_status varchar(24),
  last_error text,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK((frequency <> 'weekly') OR send_weekday IS NOT NULL),
  CHECK((frequency <> 'monthly') OR send_monthday IS NOT NULL)
);
CREATE INDEX report_schedules_due_idx
  ON report_schedules(next_run_at) WHERE is_active;
CREATE INDEX report_schedules_company_idx
  ON report_schedules(company_id,created_at DESC);

ALTER TABLE visits ADD COLUMN IF NOT EXISTS sales_transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX IF NOT EXISTS visits_sales_transaction_uidx
  ON visits(company_id,sales_transaction_id) WHERE sales_transaction_id IS NOT NULL;
ALTER TABLE bonus_ledger ADD COLUMN IF NOT EXISTS sales_transaction_id uuid REFERENCES sales_transactions(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS bonus_ledger_sales_transaction_idx
  ON bonus_ledger(company_id,sales_transaction_id) WHERE sales_transaction_id IS NOT NULL;

ALTER TABLE integration_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_location_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales_transaction_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_attributions ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_sync_cursors ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_failures ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_conversions ENABLE ROW LEVEL SECURITY;
ALTER TABLE referral_programs ENABLE ROW LEVEL SECURITY;
ALTER TABLE referral_attributions ENABLE ROW LEVEL SECURITY;
ALTER TABLE referral_rewards ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics_daily_facts ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics_customer_features ENABLE ROW LEVEL SECURITY;
ALTER TABLE report_schedules ENABLE ROW LEVEL SECURITY;

DO $$
DECLARE table_name text;
BEGIN
  FOREACH table_name IN ARRAY ARRAY[
    'integration_connections','integration_location_mappings','sales_transactions',
    'sales_transaction_items','payments','customer_events','customer_attributions',
    'integration_sync_cursors','integration_jobs','integration_failures',
    'campaign_conversions','referral_programs','referral_attributions','referral_rewards',
    'analytics_daily_facts','analytics_customer_features','report_schedules'
  ]
  LOOP
    EXECUTE format(
      'CREATE POLICY tenant_isolation ON %I USING (company_id = nullif(current_setting(''app.company_id'', true), '''')::uuid) WITH CHECK (company_id = nullif(current_setting(''app.company_id'', true), '''')::uuid)',
      table_name
    );
  END LOOP;
END $$;
