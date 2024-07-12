SET statement_timeout = 0;

--bun:split

ALTER TABLE namespaces ADD COLUMN IF NOT EXISTS mirrored BOOLEAN default false;
