SET statement_timeout = 0;
--bun:split
ALTER TABLE agent_instance_session_histories
ADD COLUMN IF NOT EXISTS feedback_reason TEXT NULL;
