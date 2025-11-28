SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_instance_session_histories ADD COLUMN IF NOT EXISTS uuid VARCHAR(36) NOT NULL DEFAULT gen_random_uuid()::text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_instance_session_history_uuid ON agent_instance_session_histories (uuid);

--bun:split

ALTER TABLE agent_instance_session_histories ADD COLUMN IF NOT EXISTS feedback VARCHAR(8) NOT NULL DEFAULT 'none';

--bun:split

ALTER TABLE agent_instance_session_histories ADD COLUMN IF NOT EXISTS is_rewritten BOOLEAN NOT NULL DEFAULT FALSE;