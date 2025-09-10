SET statement_timeout = 0;

--bun:split

ALTER TABLE account_users ADD COLUMN IF NOT EXISTS negative_balance_warn_at timestamp with time zone;
