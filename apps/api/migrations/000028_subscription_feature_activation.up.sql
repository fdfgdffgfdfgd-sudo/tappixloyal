-- Repair subscriptions changed through Platform Console before plan defaults
-- were applied automatically. The latest active subscription is authoritative.
WITH current_plans AS (
  SELECT DISTINCT ON (company_id)
    company_id,
    CASE lower(plan_code)
      WHEN 'business' THEN 'growth'
      WHEN 'enterprise' THEN 'pro'
      ELSE lower(plan_code)
    END AS plan_code
  FROM subscriptions
  WHERE status IN ('trial','active','past_due')
  ORDER BY company_id,created_at DESC
), desired AS (
  SELECT cp.company_id,m.code,
    CASE
      WHEN m.code IN ('core','crm','loyalty','reviews') THEN true
      WHEN cp.plan_code IN ('growth','pro') AND m.code IN ('analytics','website','booking','email','sms','telegram','partnerships') THEN true
      WHEN cp.plan_code='pro' AND m.code='api' THEN true
      ELSE false
    END AS enabled
  FROM current_plans cp CROSS JOIN modules m
)
INSERT INTO company_modules(company_id,module_code,enabled)
SELECT company_id,code,enabled FROM desired
ON CONFLICT(company_id,module_code)
DO UPDATE SET enabled=excluded.enabled,updated_at=now();
