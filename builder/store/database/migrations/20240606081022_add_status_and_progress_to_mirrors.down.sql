SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors DROP COLUMN IF EXISTS status;

--bun:split

ALTER TABLE mirrors DROP COLUMN IF EXISTS progress;

--bun:split

DROP INDEX IF EXISTS idx_mirrors_status;