SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_unique_repositories_repo_id_runtime_framework_id;

--bun:split

DROP INDEX IF EXISTS idx_repositories_runtime_framework_id;
