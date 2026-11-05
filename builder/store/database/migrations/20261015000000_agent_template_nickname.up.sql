SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_templates ADD COLUMN IF NOT EXISTS nickname TEXT;

--bun:split

ALTER TABLE agent_templates ALTER COLUMN nickname TYPE TEXT;
