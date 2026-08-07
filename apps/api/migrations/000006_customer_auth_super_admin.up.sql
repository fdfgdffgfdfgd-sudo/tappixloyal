ALTER TABLE customers ADD COLUMN IF NOT EXISTS pin_hash text;

INSERT INTO users(id,company_id,first_name,last_name,email,password_hash,role,status)
VALUES('30000000-0000-0000-0000-000000000099',NULL,'Армат','Tappix','admin@tappix.kz',crypt('Admin2026!',gen_salt('bf')),'super_admin','active')
ON CONFLICT DO NOTHING;
