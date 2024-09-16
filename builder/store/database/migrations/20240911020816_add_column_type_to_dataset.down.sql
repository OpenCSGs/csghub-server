SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_datasets_type;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS type;
