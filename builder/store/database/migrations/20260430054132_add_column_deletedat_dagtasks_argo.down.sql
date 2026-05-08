SET statement_timeout = 0;

--bun:split

ALTER TABLE argo_workflows DROP COLUMN IF EXISTS dag_tasks;

--bun:split

ALTER TABLE argo_workflows DROP COLUMN IF EXISTS deleted_at;
