SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors ADD COLUMN IF NOT EXISTS next_execution_timestamp TIMESTAMP;

