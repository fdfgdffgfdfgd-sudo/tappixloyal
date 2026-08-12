DROP INDEX IF EXISTS customers_company_code_unique;
DROP TRIGGER IF EXISTS customers_assign_code ON customers;
DROP FUNCTION IF EXISTS assign_customer_code();
ALTER TABLE customers DROP COLUMN IF EXISTS customer_code;
