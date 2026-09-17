SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS reasoning_token DOUBLE PRECISION DEFAULT 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS reasoning_token DOUBLE PRECISION DEFAULT 0;

--bun:split

ALTER TABLE account_statistics ADD COLUMN IF NOT EXISTS reasoning_token DOUBLE PRECISION DEFAULT 0;

--bun:split

-- account_statements_partitioned is saas-only (created by the history-archive
-- migration via LIKE account_statements, so it predates this column); the
-- sync trigger and backfill INSERT ... SELECT src.* require matching column
-- counts. ALTER TABLE IF EXISTS keeps CE/EE databases without the table safe.
ALTER TABLE IF EXISTS account_statements_partitioned ADD COLUMN IF NOT EXISTS reasoning_token DOUBLE PRECISION DEFAULT 0;
