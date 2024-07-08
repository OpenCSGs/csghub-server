SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements ALTER COLUMN op_uid TYPE VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS value_type int;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS resource_id VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS resource_name VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS sku_id bigint;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS recorded_at timestamp with time zone;
