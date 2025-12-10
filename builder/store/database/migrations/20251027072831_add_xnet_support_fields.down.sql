SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS xnet_enabled;

--bun:split

ALTER TABLE lfs_meta_objects DROP COLUMN IF EXISTS xnet_used;