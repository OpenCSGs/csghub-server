SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_git_path ON repositories(git_path);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_repository_type ON repositories(repository_type);

--bun:split

DROP INDEX IF EXISTS idx_repositories_repository_type_path; 

--bun:split

DROP INDEX IF EXISTS idx_repositories_git_path; 

