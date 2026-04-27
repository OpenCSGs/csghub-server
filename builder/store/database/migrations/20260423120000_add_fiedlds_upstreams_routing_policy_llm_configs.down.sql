SET statement_timeout = 0;

--bun:split

-- Rollback for consolidated migration:
-- Drop upstreams + routing_policy.
ALTER TABLE llm_configs DROP COLUMN IF EXISTS upstreams;
ALTER TABLE llm_configs DROP COLUMN IF EXISTS routing_policy;
