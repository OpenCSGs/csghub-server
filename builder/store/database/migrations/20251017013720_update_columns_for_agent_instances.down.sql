SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_instances DROP COLUMN IF EXISTS name;

--bun:split

ALTER TABLE agent_instances DROP COLUMN IF EXISTS description;

--bun:split

DROP INDEX IF EXISTS idx_agent_instances_name;

--bun:split

DROP INDEX IF EXISTS idx_agent_instances_type_name;

--bun:split

DROP INDEX IF EXISTS idx_agent_instances_description;

--bun:split

DROP INDEX IF EXISTS idx_agent_instances_updated_at;

--bun:split

ALTER TABLE agent_instances DROP CONSTRAINT IF EXISTS unique_agent_instances_type_content_id;