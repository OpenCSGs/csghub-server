SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_mirrors_source_url ON mirrors (source_url);
