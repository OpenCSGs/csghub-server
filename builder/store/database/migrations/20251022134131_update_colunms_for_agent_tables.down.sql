SET statement_timeout = 0;

--bun:split

-- Clean up system-built agent instances
DELETE FROM agent_instances WHERE built_in = true;

--bun:split

ALTER TABLE agent_instances DROP COLUMN IF EXISTS built_in;

--bun:split

ALTER TABLE agent_instances DROP COLUMN IF EXISTS metadata;

--bun:split

ALTER TABLE agent_instance_sessions DROP COLUMN IF EXISTS last_turn;

--bun:split

DROP INDEX IF EXISTS idx_agent_instance_session_last_turn;

--bun:split

ALTER TABLE agent_instance_session_histories DROP COLUMN IF EXISTS turn;

--bun:split

DROP INDEX IF EXISTS idx_agent_instance_session_history_session_turn_request;
