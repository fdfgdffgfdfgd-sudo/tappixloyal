ALTER TABLE customers ADD COLUMN IF NOT EXISTS pin_hash text;

INSERT INTO customers(company_id,first_name,last_name,phone,birthday,pin_hash,total_points)
VALUES('10000000-0000-0000-0000-000000000001','Мадина','Тест','+7 700 333 33 33','1998-05-12',crypt('1234',gen_salt('bf')),20)
ON CONFLICT(company_id,phone) WHERE deleted_at IS NULL
DO UPDATE SET pin_hash=excluded.pin_hash;

INSERT INTO users(id,company_id,first_name,last_name,email,password_hash,role,status)
VALUES('30000000-0000-0000-0000-000000000099',NULL,'Армат','Tappix','admin@tappix.kz',crypt('Admin2026!',gen_salt('bf')),'super_admin','active')
ON CONFLICT DO NOTHING;
