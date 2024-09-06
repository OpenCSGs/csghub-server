SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_repository_id_path ON lfs_locks (repository_id, path);

