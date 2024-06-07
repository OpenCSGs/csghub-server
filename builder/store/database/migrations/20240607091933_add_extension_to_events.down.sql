SET statement_timeout = 0;

--bun:split

ALTER TABLE events DROP COLUMN IF EXISTS extension;
