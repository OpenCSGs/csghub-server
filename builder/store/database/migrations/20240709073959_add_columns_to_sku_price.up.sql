SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices ADD COLUMN IF NOT EXISTS sku_unit_type VARCHAR;

--bun:split

ALTER TABLE account_prices ADD COLUMN IF NOT EXISTS sku_price_currency VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS sku_unit bigint;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS sku_unit_type VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS sku_price_currency VARCHAR;

--bun:split

ALTER TABLE account_meterings ADD COLUMN IF NOT EXISTS sku_unit_type VARCHAR;
