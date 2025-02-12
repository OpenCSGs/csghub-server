SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS csg_path;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS hf_path;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS ms_path;
