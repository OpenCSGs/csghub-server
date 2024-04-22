SET statement_timeout = 0;

--bun:split

ALTER TABLE space_resources ADD COLUMN IF NOT EXISTS resources VARCHAR;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_space_resources_name ON space_resources (name);

--bun:split

ALTER TABLE space_resources DROP COLUMN IF EXISTS cpu;

--bun:split

ALTER TABLE space_resources DROP COLUMN IF EXISTS gpu;

--bun:split

ALTER TABLE space_resources DROP COLUMN IF EXISTS memory;

--bun:split

ALTER TABLE space_resources DROP COLUMN IF EXISTS disk;
