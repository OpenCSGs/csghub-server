SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS balance_value_prev DOUBLE PRECISION;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS token_plan_period_id bigint;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS recorded_at_unix bigint;

--bun:split

-- account_statements_partitioned is saas-only (created by the history-archive
-- migration via LIKE account_statements, so it predates these columns); the
-- sync trigger and backfill INSERT ... SELECT src.* require matching column
-- counts. ALTER TABLE IF EXISTS keeps CE/EE databases without the table safe.
ALTER TABLE IF EXISTS account_statements_partitioned
	ADD COLUMN IF NOT EXISTS balance_value_prev DOUBLE PRECISION,
	ADD COLUMN IF NOT EXISTS token_plan_period_id bigint,
	ADD COLUMN IF NOT EXISTS recorded_at_unix bigint;
