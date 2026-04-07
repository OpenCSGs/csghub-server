SET statement_timeout = 0;

--bun:split

-- Hard-delete all soft-deleted agent_instance_session_histories before dropping column
DELETE FROM agent_instance_session_histories WHERE deleted_at IS NOT NULL;

--bun:split

-- Hard-delete all soft-deleted agent_instance_sessions before dropping column
DELETE FROM agent_instance_sessions WHERE deleted_at IS NOT NULL;

--bun:split

-- Hard-delete all soft-deleted agent_instance_tasks before dropping column
DELETE FROM agent_instance_tasks WHERE deleted_at IS NOT NULL;

--bun:split

-- Hard-delete all soft-deleted agent_instances before dropping column
DELETE FROM agent_instances WHERE deleted_at IS NOT NULL;

--bun:split

-- Hard-delete all soft-deleted agent_templates before dropping column
DELETE FROM agent_templates WHERE deleted_at IS NOT NULL;

--bun:split

ALTER TABLE agent_instance_session_histories DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE agent_instance_sessions DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE agent_instance_tasks DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE agent_instances DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE agent_templates DROP COLUMN IF EXISTS deleted_at;
