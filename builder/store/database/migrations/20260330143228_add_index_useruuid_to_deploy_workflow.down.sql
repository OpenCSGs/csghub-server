SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_deploy_useruuid_type_status ON deploys;

--bun:split

DROP INDEX IF EXISTS idx_workflow_useruuid_status ON argo_workflows;
