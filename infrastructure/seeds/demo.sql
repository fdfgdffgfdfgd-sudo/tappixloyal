-- Disposable local/CI fixtures. Never execute this file in production.
INSERT INTO companies (id,name,slug,email) VALUES
('10000000-0000-0000-0000-000000000001','Dentline','dentline','hello@dentline.kz'),
('10000000-0000-0000-0000-000000000002','DocMed','docmed','hello@docmed.kz') ON CONFLICT DO NOTHING;

INSERT INTO branches (id,company_id,name,address) VALUES
('20000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001','Левый берег','Астана, проспект Кабанбай батыра, 42'),
('20000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','Главный филиал','Алматы, проспект Абая, 10') ON CONFLICT DO NOTHING;

INSERT INTO users (id,company_id,first_name,last_name,email,password_hash,role,status) VALUES
('30000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001','Армат','','armat@tappix.kz',crypt('Tappix2026!',gen_salt('bf')),'company_owner','active'),
('30000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','Демо','DocMed','owner@docmed.kz',crypt('DocMed2026!',gen_salt('bf')),'company_owner','active'),
('30000000-0000-0000-0000-000000000099',NULL,'Армат','Tappix','admin@tappix.kz',crypt('Admin2026!',gen_salt('bf')),'super_admin','active') ON CONFLICT DO NOTHING;

INSERT INTO customers (company_id,first_name,last_name,phone,birthday,pin_hash,total_points,total_visits,level,created_at) VALUES
('10000000-0000-0000-0000-000000000001','Арман','Садыков','+7 701 234 56 78',NULL,NULL,250,12,'gold',now()-interval '2 months'),
('10000000-0000-0000-0000-000000000001','Алия','Нуржанова','+7 777 451 22 10',NULL,NULL,160,8,'silver',now()-interval '1 month'),
('10000000-0000-0000-0000-000000000001','Данияр','Ахметов','+7 702 889 14 03',NULL,NULL,100,5,'basic',now()-interval '12 days'),
('10000000-0000-0000-0000-000000000001','Жанна','Касымова','+7 705 108 76 44',NULL,NULL,60,3,'basic',now()),
('10000000-0000-0000-0000-000000000001','Мадина','Тест','+7 700 333 33 33','1998-05-12',crypt('1234',gen_salt('bf')),20,0,'basic',now()),
('10000000-0000-0000-0000-000000000002','Клиент','DocMed','+7 700 222 22 22',NULL,NULL,0,0,'basic',now())
ON CONFLICT(company_id,phone) WHERE deleted_at IS NULL DO UPDATE SET pin_hash=coalesce(excluded.pin_hash,customers.pin_hash);

INSERT INTO company_modules(company_id,module_code,enabled)
SELECT c.id,m.code,CASE WHEN c.slug='dentline' THEN m.code IN ('core','crm','loyalty','analytics','reviews','email') ELSE m.code IN ('core','crm','loyalty') END
FROM companies c CROSS JOIN modules m WHERE c.slug IN ('dentline','docmed')
ON CONFLICT(company_id,module_code) DO UPDATE SET enabled=excluded.enabled;

INSERT INTO subscriptions(company_id,plan_code,status,amount,currency,billing_period,current_period_ends_at) VALUES
('10000000-0000-0000-0000-000000000001','pro','active',24990,'KZT','monthly',now()+interval '30 days'),
('10000000-0000-0000-0000-000000000002','starter','trial',0,'KZT','monthly',now()+interval '7 days') ON CONFLICT DO NOTHING;

INSERT INTO devices(company_id,branch_id,kind,name,destination) VALUES
('10000000-0000-0000-0000-000000000001','20000000-0000-0000-0000-000000000001','nfc','Reception NFC','join'),
('10000000-0000-0000-0000-000000000001','20000000-0000-0000-0000-000000000001','qr','Стойка регистрации','join') ON CONFLICT DO NOTHING;
