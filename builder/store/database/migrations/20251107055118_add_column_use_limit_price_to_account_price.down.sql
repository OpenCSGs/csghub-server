SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS use_limit_price;
