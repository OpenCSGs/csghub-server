SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices ADD COLUMN IF NOT EXISTS sku_cached_price bigint DEFAULT -1;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS prompt_cached_token DOUBLE PRECISION DEFAULT 0;

--bun:split

ALTER TABLE account_bills ADD COLUMN IF NOT EXISTS prompt_cached_token DOUBLE PRECISION DEFAULT 0;

