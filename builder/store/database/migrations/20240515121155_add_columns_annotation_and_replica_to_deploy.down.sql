SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS deploy_name;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS user_id;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS model_id;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS repo_id;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS runtime_framework;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS annotation;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS min_replica;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS max_replica;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS svc_name;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS endpoint;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS cost_per_hour;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS cluster_id;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS secure_level;
