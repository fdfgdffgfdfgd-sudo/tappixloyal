ALTER TABLE companies ADD COLUMN IF NOT EXISTS legal_name varchar(200);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS category varchar(80);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS whatsapp varchar(80);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS instagram varchar(200);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS city varchar(120);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS currency varchar(3) NOT NULL DEFAULT 'KZT';
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone varchar(32);
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password boolean NOT NULL DEFAULT false;
