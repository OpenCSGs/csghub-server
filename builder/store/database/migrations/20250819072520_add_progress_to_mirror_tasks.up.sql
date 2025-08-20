SET statement_timeout = 0;

--bun:split

ALTER TABLE mirror_tasks ADD COLUMN IF NOT EXISTS progress INTEGER DEFAULT 0 NOT NULL;
