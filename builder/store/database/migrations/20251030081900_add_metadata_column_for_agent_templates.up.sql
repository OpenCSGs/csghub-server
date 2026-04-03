SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_templates ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';
