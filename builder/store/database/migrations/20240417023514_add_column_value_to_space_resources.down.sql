SET statement_timeout = 0;

--bun:split

ALTER TABLE space_resources DROP COLUMN IF EXISTS resources;

--bun:split

DROP INDEX IF EXISTS idx_space_resources_name;

--bun:split

ALTER TABLE space_resources ADD COLUMN IF NOT EXISTS cpu INT;

--bun:split

ALTER TABLE space_resources ADD COLUMN IF NOT EXISTS gpu INT;

--bun:split

ALTER TABLE space_resources ADD COLUMN IF NOT EXISTS memory INT;

--bun:split

ALTER TABLE space_resources ADD COLUMN IF NOT EXISTS disk INT;
