SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_instances DROP CONSTRAINT IF EXISTS unique_agent_instances_type_content_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_instances_type_content_id_unique_active ON agent_instances(type, content_id) WHERE deleted_at IS NULL;