SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS source;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS sync_status;

--bun:split

DROP INDEX IF EXISTS idx_repositories_source;