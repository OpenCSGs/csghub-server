SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_templates ADD COLUMN IF NOT EXISTS name VARCHAR(255);

--bun:split

ALTER TABLE agent_templates ADD COLUMN IF NOT EXISTS description VARCHAR(500);

--bun:split

CREATE INDEX IF NOT EXISTS idx_agent_templates_name ON agent_templates (name);
