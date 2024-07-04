SET statement_timeout = 0;

--bun:split

ALTER TABLE sync_client_settings ADD COLUMN IF NOT EXISTS is_default boolean default false;

