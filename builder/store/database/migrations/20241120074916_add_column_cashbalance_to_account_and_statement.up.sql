SET statement_timeout = 0;

--bun:split

ALTER TABLE account_users ADD COLUMN IF NOT EXISTS cash_balance DOUBLE PRECISION DEFAULT 0;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS balance_type VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS balance_value DOUBLE PRECISION;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS is_cancel BOOLEAN DEFAULT FALSE;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS event_value DOUBLE PRECISION;

--bun:split

DROP INDEX IF EXISTS idx_unique_account_statement_event_uuid;
