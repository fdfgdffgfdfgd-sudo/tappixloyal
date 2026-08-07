INSERT INTO users (id, company_id, first_name, last_name, email, password_hash, role, status)
VALUES ('30000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001','Армат','','armat@tappix.kz',crypt('Tappix2026!',gen_salt('bf')),'company_owner','active')
ON CONFLICT DO NOTHING;
