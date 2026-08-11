SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS sku_cached_price;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS prompt_cached_token;

--bun:split

ALTER TABLE account_bills DROP COLUMN IF EXISTS prompt_cached_token;
