SET statement_timeout = 0;

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS sku_unit_type;

--bun:split

ALTER TABLE account_prices DROP COLUMN IF EXISTS sku_price_currency;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS sku_unit;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS sku_unit_type;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS sku_price_currency;

--bun:split

ALTER TABLE account_meterings DROP COLUMN IF EXISTS sku_unit_type;
