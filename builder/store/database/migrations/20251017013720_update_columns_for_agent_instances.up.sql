SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_instances ADD COLUMN IF NOT EXISTS name VARCHAR(255);

--bun:split

ALTER TABLE agent_instances ADD COLUMN IF NOT EXISTS description VARCHAR(500);

--bun:split

CREATE INDEX IF NOT EXISTS idx_agent_instances_name ON agent_instances (LOWER(name));

--bun:split

CREATE INDEX IF NOT EXISTS idx_agent_instances_type_name ON agent_instances (type, LOWER(name));

--bun:split

CREATE INDEX IF NOT EXISTS idx_agent_instances_description ON agent_instances (LOWER(description));

--bun:split

CREATE INDEX IF NOT EXISTS idx_agent_instances_updated_at ON agent_instances (updated_at);

--bun:split

ALTER TABLE agent_instances ADD CONSTRAINT unique_agent_instances_type_content_id UNIQUE (type, content_id);