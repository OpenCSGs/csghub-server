SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_deploys_reconcile;

--bun:split

CREATE INDEX idx_deploys_reconcile ON deploys (status_update_at, status);

--bun:split

DROP INDEX IF EXISTS idx_argo_workflows_reconcile;

--bun:split

CREATE INDEX idx_argo_workflows_reconcile ON argo_workflows (status_update_at, status);
