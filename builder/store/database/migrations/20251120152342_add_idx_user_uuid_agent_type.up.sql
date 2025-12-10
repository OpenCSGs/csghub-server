SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_agent_instances_user_uuid_type ON agent_instances (user_uuid, type);
