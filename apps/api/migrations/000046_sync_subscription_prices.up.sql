-- Keep existing subscriptions aligned with the public commercial catalogue.
-- The catalogue migration updated plans_v2, but historic subscriptions kept
-- obsolete amounts such as 99 900 KZT.
UPDATE subscriptions s
SET amount = CASE
      WHEN s.billing_period = 'yearly' THEN coalesce(p.annual_price, p.monthly_price * 10)
      ELSE p.monthly_price
    END,
    currency = p.currency,
    updated_at = now()
FROM plans_v2 p
WHERE p.code = s.plan_code
  AND (
    s.amount IS DISTINCT FROM CASE
      WHEN s.billing_period = 'yearly' THEN coalesce(p.annual_price, p.monthly_price * 10)
      ELSE p.monthly_price
    END
    OR s.currency IS DISTINCT FROM p.currency
  );
