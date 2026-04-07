SET statement_timeout = 0;

--bun:split

ALTER TABLE payment_stripes DROP COLUMN IF EXISTS op_uuid;
