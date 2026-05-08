SET statement_timeout = 0;

--bun:split

ALTER TABLE argo_workflows ADD COLUMN IF NOT EXISTS dag_tasks VARCHAR;

--bun:split

ALTER TABLE argo_workflows ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;
