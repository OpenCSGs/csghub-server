SET statement_timeout = 0;

--bun:split

ALTER TABLE argo_workflows DROP CONSTRAINT IF EXISTS unique_argo_workflow_taskid;
