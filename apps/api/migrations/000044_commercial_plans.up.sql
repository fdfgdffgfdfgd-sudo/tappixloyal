ALTER TABLE plans_v2 ADD COLUMN IF NOT EXISTS annual_price numeric(12,2);
ALTER TABLE plans_v2 ADD COLUMN IF NOT EXISTS trial_days integer NOT NULL DEFAULT 7 CHECK(trial_days >= 0);
ALTER TABLE plans_v2 ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';
ALTER TABLE plans_v2 ADD COLUMN IF NOT EXISTS highlighted boolean NOT NULL DEFAULT false;

UPDATE plans_v2 SET
  name = CASE code WHEN 'starter' THEN 'Start' WHEN 'growth' THEN 'Pro' WHEN 'pro' THEN 'Business' ELSE name END,
  monthly_price = CASE code WHEN 'starter' THEN 7990 WHEN 'growth' THEN 14990 WHEN 'pro' THEN 24990 ELSE monthly_price END,
  annual_price = CASE code WHEN 'starter' THEN 79900 WHEN 'growth' THEN 149900 WHEN 'pro' THEN 249900 ELSE annual_price END,
  trial_days = 7,
  description = CASE code
    WHEN 'starter' THEN 'Для малого бизнеса и первой точки'
    WHEN 'growth' THEN 'Для растущего бизнеса и автоматизации'
    WHEN 'pro' THEN 'Для сетей и нескольких филиалов'
    ELSE description END,
  highlighted = code = 'growth',
  updated_at = now()
WHERE code IN ('starter','growth','pro');
