SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS source VARCHAR default 'local';

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS sync_status VARCHAR;

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_source ON repositories (source);