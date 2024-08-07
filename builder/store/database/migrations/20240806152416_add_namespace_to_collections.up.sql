SET statement_timeout = 0;

--bun:split

ALTER TABLE collections ADD COLUMN IF NOT EXISTS namespace VARCHAR;

--bun:split
CREATE INDEX IF NOT EXISTS idx_collection_namespace ON collections (namespace);
