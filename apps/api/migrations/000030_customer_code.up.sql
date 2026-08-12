ALTER TABLE customers ADD COLUMN IF NOT EXISTS customer_code char(6);

CREATE OR REPLACE FUNCTION assign_customer_code() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE candidate char(6);
BEGIN
  IF NEW.customer_code IS NOT NULL THEN RETURN NEW; END IF;
  LOOP
    candidate := lpad(((get_byte(gen_random_bytes(3),0)::integer * 65536 + get_byte(gen_random_bytes(3),1)::integer * 256 + get_byte(gen_random_bytes(3),2)::integer) % 1000000)::text,6,'0');
    EXIT WHEN NOT EXISTS (SELECT 1 FROM customers WHERE company_id=NEW.company_id AND customer_code=candidate);
  END LOOP;
  NEW.customer_code := candidate;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS customers_assign_code ON customers;
CREATE TRIGGER customers_assign_code BEFORE INSERT ON customers FOR EACH ROW EXECUTE FUNCTION assign_customer_code();

DO $$ DECLARE row_item record; candidate char(6); BEGIN
  FOR row_item IN SELECT id FROM customers WHERE customer_code IS NULL LOOP
    LOOP
      candidate := lpad(((get_byte(gen_random_bytes(3),0)::integer * 65536 + get_byte(gen_random_bytes(3),1)::integer * 256 + get_byte(gen_random_bytes(3),2)::integer) % 1000000)::text,6,'0');
      EXIT WHEN NOT EXISTS (SELECT 1 FROM customers c JOIN customers target ON target.id=row_item.id WHERE c.company_id=target.company_id AND c.customer_code=candidate);
    END LOOP;
    UPDATE customers SET customer_code=candidate WHERE id=row_item.id;
  END LOOP;
END $$;

ALTER TABLE customers ALTER COLUMN customer_code SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS customers_company_code_unique ON customers(company_id,customer_code);
