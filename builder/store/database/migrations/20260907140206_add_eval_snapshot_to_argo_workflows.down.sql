SET statement_timeout = 0;

--bun:split

ALTER TABLE argo_workflows DROP COLUMN IF EXISTS repo_revisions;

--bun:split

ALTER TABLE argo_workflows DROP COLUMN IF EXISTS dataset_revisions;

--bun:split

ALTER TABLE argo_workflows DROP COLUMN IF EXISTS framework_config;

--bun:split

ALTER TABLE argo_workflows DROP COLUMN IF EXISTS hardware;
