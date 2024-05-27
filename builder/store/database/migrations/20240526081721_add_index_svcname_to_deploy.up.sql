SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_deploy_svcname ON deploys (svc_name);

