SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS lfs_objects_size bigint;
