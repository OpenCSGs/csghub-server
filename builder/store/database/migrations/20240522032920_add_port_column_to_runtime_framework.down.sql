SET statement_timeout = 0;

--bun:split

ALTER TABLE runtime_frameworks DROP COLUMN IF EXISTS container_port;

--bun:split

ALTER TABLE runtime_frameworks DROP COLUMN IF EXISTS frame_cpu_image;

--bun:split

DROP INDEX IF EXISTS idx_runtime_frameworks_name;
