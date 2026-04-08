SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_deploy_useruuid_type_status ON deploys (user_uuid, type, status);


--bun:split

CREATE INDEX IF NOT EXISTS idx_workflow_useruuid_status ON argo_workflows (user_uuid, status);
