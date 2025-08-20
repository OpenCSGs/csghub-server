SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors DROP COLUMN IF EXISTS retry_count;

--bun:split

ALTER TABLE mirrors DROP COLUMN IF EXISTS remote_updated_at;
