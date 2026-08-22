UPDATE subscriptions SET plan_code=CASE lower(plan_code)
  WHEN 'start' THEN 'starter' WHEN 'starter' THEN 'starter' WHEN 'growth' THEN 'growth'
  WHEN 'business' THEN 'pro' WHEN 'enterprise' THEN 'pro' ELSE lower(plan_code) END;

INSERT INTO plan_entitlements(plan_code,code,enabled,limit_value) VALUES
('starter','branches',true,1),('starter','integrations',false,0),('starter','export',false,0),
('growth','branches',true,3),('growth','integrations',true,1),('growth','export',true,NULL),
('pro','branches',true,100),('pro','integrations',true,10),('pro','export',true,NULL)
ON CONFLICT(plan_code,code) DO UPDATE SET enabled=excluded.enabled,limit_value=excluded.limit_value;

CREATE INDEX IF NOT EXISTS subscriptions_company_created_idx ON subscriptions(company_id,created_at DESC);
CREATE INDEX IF NOT EXISTS integration_jobs_connection_created_idx ON integration_jobs(company_id,connection_id,created_at DESC);
