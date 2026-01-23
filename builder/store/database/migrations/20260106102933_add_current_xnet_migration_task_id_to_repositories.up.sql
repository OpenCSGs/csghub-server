SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS current_xnet_migration_task_id BIGINT;

