SET statement_timeout = 0;

--bun:split

ALTER TABLE account_users ADD COLUMN IF NOT EXISTS low_balance_warn DOUBLE PRECISION NOT NULL DEFAULT 0;
