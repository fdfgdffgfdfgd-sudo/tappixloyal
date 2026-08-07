INSERT INTO subscriptions(company_id,plan_code,status,amount,currency,billing_period,current_period_ends_at)
VALUES
('10000000-0000-0000-0000-000000000001','Business','active',29900,'KZT','monthly',now()+interval '30 days'),
('10000000-0000-0000-0000-000000000002','Starter','trial',0,'KZT','monthly',now()+interval '14 days')
ON CONFLICT DO NOTHING;

INSERT INTO devices(company_id,branch_id,kind,name,destination)
VALUES
('10000000-0000-0000-0000-000000000001','20000000-0000-0000-0000-000000000001','nfc','Reception NFC','join'),
('10000000-0000-0000-0000-000000000001','20000000-0000-0000-0000-000000000001','qr','Стойка регистрации','join')
ON CONFLICT DO NOTHING;
