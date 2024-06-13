SET statement_timeout = 0;

--bun:split

ALTER TABLE namespaces DROP COLUMN IF EXISTS mirrored;
