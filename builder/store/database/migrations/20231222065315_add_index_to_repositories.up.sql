SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_path ON repositories(path);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_user_id ON repositories(user_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_git_path ON repositories(git_path);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_repository_type ON repositories(repository_type);