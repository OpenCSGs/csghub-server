SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS cluster_id VARCHAR(255) NOT NULL DEFAULT 'config';
