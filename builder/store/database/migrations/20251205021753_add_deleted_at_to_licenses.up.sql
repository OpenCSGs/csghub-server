SET statement_timeout = 0;

--bun:split

ALTER TABLE licenses ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;
