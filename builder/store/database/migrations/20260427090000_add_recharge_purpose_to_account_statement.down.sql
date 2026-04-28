ALTER TABLE account_statements DROP COLUMN IF EXISTS purpose;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS purpose_desc;
