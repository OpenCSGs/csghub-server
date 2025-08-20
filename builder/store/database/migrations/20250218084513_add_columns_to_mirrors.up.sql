SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors ADD COLUMN IF NOT EXISTS retry_count INT DEFAULT 0;

--bun:split

ALTER TABLE mirrors ADD COLUMN IF NOT EXISTS remote_updated_at TIMESTAMP;

