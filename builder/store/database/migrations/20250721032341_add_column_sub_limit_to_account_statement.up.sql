SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS quota DOUBLE PRECISION;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS sub_bill_id BIGINT;
