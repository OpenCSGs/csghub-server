SET statement_timeout = 0;

--bun:split

ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS container_port INT;

--bun:split

ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS frame_cpu_image VARCHAR;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_runtime_frameworks_name ON runtime_frameworks (frame_name);
