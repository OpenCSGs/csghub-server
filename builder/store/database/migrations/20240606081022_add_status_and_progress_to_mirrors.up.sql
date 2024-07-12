SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors ADD COLUMN IF NOT EXISTS status VARCHAR(20);

--bun:split

ALTER TABLE mirrors ADD COLUMN IF NOT EXISTS progress INT;

--bun:split

CREATE INDEX IF NOT EXISTS idx_mirrors_status ON mirrors (status);