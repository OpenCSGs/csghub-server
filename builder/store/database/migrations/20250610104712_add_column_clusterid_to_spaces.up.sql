SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS cluster_id VARCHAR;
