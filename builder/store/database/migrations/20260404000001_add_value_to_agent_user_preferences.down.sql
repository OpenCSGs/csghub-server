SET statement_timeout = 0;
--bun:split
ALTER TABLE agent_user_preferences DROP COLUMN IF EXISTS value;