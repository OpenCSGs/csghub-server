SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors DROP COLUMN IF EXISTS next_execution_timestamp;

