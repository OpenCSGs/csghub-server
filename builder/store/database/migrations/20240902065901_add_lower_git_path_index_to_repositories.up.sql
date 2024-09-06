SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_repositories_lower_git_path_unique ON repositories (LOWER(git_path));

