SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories_runtime_frameworks DROP COLUMN IF EXISTS type;

--bun:split

DROP INDEX IF EXISTS idx_unique_repositories_type_repo_id_runtime_framework_id;

--bun:split

DROP INDEX IF EXISTS idx_repositories_type_runtime_framework_id;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_repositories_repo_id_runtime_framework_id ON repositories_runtime_frameworks(repo_id, runtime_framework_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repositories_runtime_framework_id ON repositories_runtime_frameworks (runtime_framework_id);
