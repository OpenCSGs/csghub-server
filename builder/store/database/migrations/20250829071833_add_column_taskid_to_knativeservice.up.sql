SET statement_timeout = 0;

--bun:split

ALTER TABLE knative_services ADD COLUMN IF NOT EXISTS task_id BIGINT;
