SET statement_timeout = 0;

--bun:split

ALTER TABLE knative_services DROP COLUMN IF EXISTS task_id;
