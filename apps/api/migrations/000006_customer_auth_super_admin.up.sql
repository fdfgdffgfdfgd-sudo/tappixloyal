ALTER TABLE customers ADD COLUMN IF NOT EXISTS pin_hash text;

-- Known customer/admin credentials moved to the development seed.
