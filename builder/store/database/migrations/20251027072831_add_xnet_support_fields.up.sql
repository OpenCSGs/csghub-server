SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS xnet_enabled BOOLEAN DEFAULT false;

--bun:split

ALTER TABLE lfs_meta_objects ADD COLUMN IF NOT EXISTS xnet_used BOOLEAN DEFAULT false;
