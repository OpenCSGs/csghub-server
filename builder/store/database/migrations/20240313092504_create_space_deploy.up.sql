SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_deploys_space_id ON deploys(space_id);

--bun:split
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_deploy_id ON deploy_tasks(deploy_id);
