SET statement_timeout = 0;

--bun:split

ALTER TABLE collections DROP COLUMN IF EXISTS namespace;

--bun:split
DROP INDEX IF EXISTS idx_collection_namespace;
