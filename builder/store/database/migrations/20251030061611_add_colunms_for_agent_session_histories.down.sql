SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_agent_instance_session_history_uuid;

ALTER TABLE agent_instance_session_histories DROP COLUMN IF EXISTS uuid;

--bun:split

ALTER TABLE agent_instance_session_histories DROP COLUMN IF EXISTS feedback;

--bun:split

ALTER TABLE agent_instance_session_histories DROP COLUMN IF EXISTS is_rewritten;
