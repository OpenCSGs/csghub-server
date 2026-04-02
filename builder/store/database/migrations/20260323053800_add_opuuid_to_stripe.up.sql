SET statement_timeout = 0;

--bun:split

ALTER TABLE payment_stripes ADD COLUMN IF NOT EXISTS op_uuid VARCHAR(255) NOT NULL DEFAULT '';

