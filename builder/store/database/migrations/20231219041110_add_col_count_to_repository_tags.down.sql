SET statement_timeout = 0;

--bun:split

ALTER TABLE repository_tags DROP COLUMN IF EXISTS count;

--bun:split
