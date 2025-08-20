SET statement_timeout = 0;

--bun:split

ALTER TABLE account_users DROP COLUMN IF EXISTS cash_balance;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS balance_type;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS balance_value;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS is_cancel;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS event_value;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_account_statement_event_uuid ON account_statements (event_uuid);
