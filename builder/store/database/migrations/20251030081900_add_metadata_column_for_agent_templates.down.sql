SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_templates DROP COLUMN IF EXISTS metadata;
