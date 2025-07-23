SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors DROP COLUMN current_task_id;
