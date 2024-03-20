SET statement_timeout = 0;

--bun:split
DROP INDEX IF EXISTS idx_deploys_space_id;

--bun:split
DROP INDEX IF EXISTS idx_deploy_tasks_deploy_id;
