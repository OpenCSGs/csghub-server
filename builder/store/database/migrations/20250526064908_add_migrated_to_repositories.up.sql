SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS migrated BOOLEAN NOT NULL DEFAULT false;
