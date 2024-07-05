SET statement_timeout = 0;

--bun:split
ALTER TABLE space_resources DROP COLUMN IF EXISTS cluster_id;
