SET statement_timeout = 0;

--bun:split
ALTER TABLE account_users DROP COLUMN IF EXISTS low_balance_warn_at;
