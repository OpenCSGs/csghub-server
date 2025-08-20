SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_account_price_skutype_resid_createdat;

--bun:split

ALTER TABLE account_prices ADD COLUMN IF NOT EXISTS sku_kind BIGINT DEFAULT 1;

--bun:split

ALTER TABLE account_prices ADD COLUMN IF NOT EXISTS quota VARCHAR;

--bun:split

ALTER TABLE account_prices ADD COLUMN IF NOT EXISTS sku_price_id BIGINT;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_price_skutype_skukind_resid_unittype_createdat ON account_prices (sku_type,sku_kind,resource_id,sku_unit_type,created_at);
