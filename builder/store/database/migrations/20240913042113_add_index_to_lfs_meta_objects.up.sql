
SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_lfs_meta_objects_repository_id_oid ON lfs_meta_objects(repository_id, oid);