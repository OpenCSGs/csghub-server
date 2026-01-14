SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS current_xnet_migration_task_id;

