SET statement_timeout = 0;

--bun:split

ALTER TABLE licenses DROP COLUMN IF EXISTS deleted_at;
