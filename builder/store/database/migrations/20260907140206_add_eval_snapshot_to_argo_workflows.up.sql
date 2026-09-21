SET statement_timeout = 0;

--bun:split

ALTER TABLE argo_workflows ADD COLUMN IF NOT EXISTS repo_revisions JSONB;

--bun:split

ALTER TABLE argo_workflows ADD COLUMN IF NOT EXISTS dataset_revisions JSONB;

--bun:split

ALTER TABLE argo_workflows ADD COLUMN IF NOT EXISTS framework_config VARCHAR;

--bun:split

ALTER TABLE argo_workflows ADD COLUMN IF NOT EXISTS hardware JSONB;
