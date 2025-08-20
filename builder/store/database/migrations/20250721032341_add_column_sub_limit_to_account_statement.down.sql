SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS quota;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS sub_bill_id;
