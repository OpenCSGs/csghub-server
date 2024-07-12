SET statement_timeout = 0;

--bun:split

ALTER TABLE account_meterings DROP COLUMN IF EXISTS sku_unit_type;
