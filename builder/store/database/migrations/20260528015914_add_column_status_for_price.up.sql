SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices ADD COLUMN sku_status bigint DEFAULT 1;

--bun:split

CREATE INDEX IF NOT EXISTS idx_acct_price_type_kind_status ON account_prices (sku_type,sku_kind,sku_status);
