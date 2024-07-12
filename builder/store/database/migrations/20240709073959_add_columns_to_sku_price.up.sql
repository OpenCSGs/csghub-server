SET statement_timeout = 0;

--bun:split

ALTER TABLE account_meterings ADD COLUMN IF NOT EXISTS sku_unit_type VARCHAR;
