SET statement_timeout = 0;

--bun:split

ALTER TABLE argo_workflows ADD COLUMN IF NOT EXISTS status_update_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;
