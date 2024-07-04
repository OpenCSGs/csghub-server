SET statement_timeout = 0;

--bun:split

ALTER TABLE sync_client_settings DROP COLUMN IF EXISTS is_default;