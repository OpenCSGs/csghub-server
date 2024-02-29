SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS likes INT DEFAULT 0;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS download_count INT DEFAULT 0;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS nickname VARCHAR;
