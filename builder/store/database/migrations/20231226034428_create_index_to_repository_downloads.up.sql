SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_repository_downloads_repository_id ON repository_downloads(repository_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repository_downloads_date ON repository_downloads(date);