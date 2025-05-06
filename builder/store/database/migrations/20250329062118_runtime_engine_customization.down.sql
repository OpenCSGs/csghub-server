SET statement_timeout = 0;

--bun:split
ALTER TABLE runtime_frameworks DROP COLUMN IF EXISTS compute_type;
ALTER TABLE runtime_frameworks DROP COLUMN IF EXISTS driver_version;
ALTER TABLE runtime_frameworks DROP COLUMN IF EXISTS description;
ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS frame_cpu_image VARCHAR DEFAULT NULL;
ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS frame_npu_image VARCHAR DEFAULT NULL;
