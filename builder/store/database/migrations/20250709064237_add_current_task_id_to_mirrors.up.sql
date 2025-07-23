SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors ADD COLUMN IF NOT EXISTS current_task_id BIGINT;
