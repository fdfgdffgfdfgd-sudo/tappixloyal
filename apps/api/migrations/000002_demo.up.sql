INSERT INTO companies (id, name, slug, email) VALUES
('10000000-0000-0000-0000-000000000001', 'Dentline', 'dentline', 'hello@dentline.kz')
ON CONFLICT DO NOTHING;

INSERT INTO branches (id, company_id, name, address) VALUES
('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'Левый берег', 'Астана, проспект Кабанбай батыра, 42')
ON CONFLICT DO NOTHING;

INSERT INTO company_modules(company_id,module_code,enabled)
SELECT '10000000-0000-0000-0000-000000000001', code, code IN ('core','crm','loyalty','analytics','reviews') FROM modules
ON CONFLICT (company_id,module_code) DO UPDATE SET enabled=excluded.enabled;

INSERT INTO customers (company_id,first_name,last_name,phone,total_points,total_visits,level,created_at) VALUES
('10000000-0000-0000-0000-000000000001','Арман','Садыков','+7 701 234 56 78',250,12,'gold',now()-interval '2 months'),
('10000000-0000-0000-0000-000000000001','Алия','Нуржанова','+7 777 451 22 10',160,8,'silver',now()-interval '1 month'),
('10000000-0000-0000-0000-000000000001','Данияр','Ахметов','+7 702 889 14 03',100,5,'basic',now()-interval '12 days'),
('10000000-0000-0000-0000-000000000001','Жанна','Касымова','+7 705 108 76 44',60,3,'basic',now())
ON CONFLICT DO NOTHING;
