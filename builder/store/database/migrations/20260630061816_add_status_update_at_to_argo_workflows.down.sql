SET statement_timeout = 0;

--bun:split

ALTER TABLE argo_workflows DROP COLUMN IF EXISTS status_update_at;
