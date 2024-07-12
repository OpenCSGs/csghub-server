SET statement_timeout = 0;

--bun:split

ALTER TABLE space_resources ADD COLUMN IF NOT EXISTS cluster_id VARCHAR;
