SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors DROP COLUMN IF EXISTS mirror_priority;

