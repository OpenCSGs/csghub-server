SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_templates ADD COLUMN is_internal boolean NOT NULL DEFAULT false;
