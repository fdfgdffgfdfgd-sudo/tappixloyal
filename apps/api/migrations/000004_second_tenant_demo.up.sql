INSERT INTO companies (id,name,slug,email) VALUES
('10000000-0000-0000-0000-000000000002','DocMed','docmed','hello@docmed.kz')
ON CONFLICT DO NOTHING;

INSERT INTO branches (id,company_id,name,address) VALUES
('20000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','Главный филиал','Алматы, проспект Абая, 10')
ON CONFLICT DO NOTHING;

INSERT INTO users (id,company_id,first_name,last_name,email,password_hash,role,status) VALUES
('30000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','Демо','DocMed','owner@docmed.kz',crypt('DocMed2026!',gen_salt('bf')),'company_owner','active')
ON CONFLICT DO NOTHING;

INSERT INTO customers (company_id,first_name,last_name,phone) VALUES
('10000000-0000-0000-0000-000000000002','Клиент','DocMed','+7 700 222 22 22')
ON CONFLICT DO NOTHING;

INSERT INTO company_modules(company_id,module_code,enabled)
SELECT '10000000-0000-0000-0000-000000000002',code,code IN ('core','crm','loyalty') FROM modules
ON CONFLICT(company_id,module_code) DO NOTHING;
