SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_repositories_path;

--bun:split

DROP INDEX IF EXISTS idx_repositories_user_id;

--bun:split

DROP INDEX IF EXISTS idx_repositories_git_path;

--bun:split

DROP INDEX IF EXISTS idx_repositories_repository_type;
