SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices ADD COLUMN IF NOT EXISTS use_limit_price BIGINT DEFAULT 0;

