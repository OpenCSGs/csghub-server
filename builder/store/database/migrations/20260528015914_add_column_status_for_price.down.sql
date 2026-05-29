SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS sku_status;

--bun:split

DROP INDEX IF EXISTS idx_acct_price_type_kind_status;

