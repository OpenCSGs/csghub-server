SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS reasoning_token;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS reasoning_token;

--bun:split

ALTER TABLE account_statistics DROP COLUMN IF EXISTS reasoning_token;

--bun:split

ALTER TABLE IF EXISTS account_statements_partitioned DROP COLUMN IF EXISTS reasoning_token;
