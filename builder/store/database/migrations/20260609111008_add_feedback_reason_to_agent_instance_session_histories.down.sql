SET statement_timeout = 0;
--bun:split
ALTER TABLE agent_instance_session_histories
DROP COLUMN IF EXISTS feedback_reason;
