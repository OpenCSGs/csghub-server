SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS deploy_name VARCHAR;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS user_id BIGINT;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS model_id BIGINT;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS repo_id BIGINT;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS runtime_framework VARCHAR;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS annotation VARCHAR;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS min_replica INT;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS max_replica INT;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS svc_name VARCHAR;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS endpoint VARCHAR;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS cost_per_hour BIGINT;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS cluster_id VARCHAR;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS secure_level INT;
