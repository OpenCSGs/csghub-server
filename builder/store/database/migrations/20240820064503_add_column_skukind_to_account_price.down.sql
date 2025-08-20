SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_account_price_skutype_skukind_resid_unittype_createdat;

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS sku_kind;

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS quota;

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS sku_price_id;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_price_skutype_resid_createdat ON account_prices (sku_type,resource_id,created_at);

