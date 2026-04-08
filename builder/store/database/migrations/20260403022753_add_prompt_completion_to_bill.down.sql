SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS prompt_token;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS completion_token;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS prompt_token;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS completion_token;
