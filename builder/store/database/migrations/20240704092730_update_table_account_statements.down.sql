SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements ALTER COLUMN op_uid TYPE bigint;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS value_type;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS resource_id;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS resource_name;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS sku_id;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS recorded_at;
