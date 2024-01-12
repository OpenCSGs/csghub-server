SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_repositories_git_path;

--bun:split

DROP INDEX IF EXISTS idx_repositories_repository_type;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_repositories_repository_type_path ON repositories(repository_type, path);

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_repositories_git_path ON repositories(git_path);
