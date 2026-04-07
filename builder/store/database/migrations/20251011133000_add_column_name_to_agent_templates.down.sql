SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_templates DROP COLUMN IF EXISTS name;

--bun:split

ALTER TABLE agent_templates DROP COLUMN IF EXISTS description;

--bun:split

DROP INDEX IF EXISTS idx_agent_templates_name;