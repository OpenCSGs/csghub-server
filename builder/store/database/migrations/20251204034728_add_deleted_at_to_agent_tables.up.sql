SET statement_timeout = 0;

--bun:split

ALTER TABLE agent_templates ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE agent_instances ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE agent_instance_sessions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE agent_instance_session_histories ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE agent_instance_tasks ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;
