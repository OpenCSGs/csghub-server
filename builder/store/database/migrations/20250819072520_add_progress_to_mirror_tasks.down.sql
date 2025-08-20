SET statement_timeout = 0;

--bun:split

ALTER TABLE mirror_tasks DROP COLUMN IF EXISTS progress;
