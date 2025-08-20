SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS csg_path VARCHAR;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS hf_path VARCHAR;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS ms_path VARCHAR;