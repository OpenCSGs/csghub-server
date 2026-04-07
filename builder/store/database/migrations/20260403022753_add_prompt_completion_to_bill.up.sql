SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS prompt_token DOUBLE PRECISION DEFAULT 0;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS completion_token DOUBLE PRECISION DEFAULT 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS prompt_token DOUBLE PRECISION DEFAULT 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS completion_token DOUBLE PRECISION DEFAULT 0;
