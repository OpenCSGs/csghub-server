SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_repositories_repository_type_lower_path;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_repositories_repository_type_path ON repositories (repository_type, path);