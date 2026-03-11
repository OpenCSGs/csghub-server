SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_deploys_created_at ON deploys(created_at);

--bun:split

CREATE INDEX IF NOT EXISTS idx_argoworkflows_submit_at ON argo_workflows(submit_time);
