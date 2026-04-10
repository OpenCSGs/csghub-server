SET statement_timeout = 0;
--bun:split
ALTER TABLE agent_user_preferences
ADD COLUMN IF NOT EXISTS value JSONB;