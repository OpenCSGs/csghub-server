SET statement_timeout = 0;

--bun:split
DELETE FROM runtime_frameworks;
DELETE FROM runtime_architectures;
ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS compute_type VARCHAR DEFAULT NULL;
ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS driver_version VARCHAR DEFAULT NULL;
ALTER TABLE runtime_frameworks DROP COLUMN IF EXISTS frame_cpu_image;
ALTER TABLE runtime_frameworks DROP COLUMN IF EXISTS frame_npu_image;
ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS description VARCHAR DEFAULT NULL;
DROP INDEX IF EXISTS idx_runtime_frameworks_name;
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_frame_driver_compute ON runtime_frameworks (frame_name, compute_type, driver_version);
ALTER TABLE runtime_architectures ADD COLUMN IF NOT EXISTS model_name VARCHAR DEFAULT NULL;
ALTER TABLE runtime_architectures ALTER COLUMN architecture_name DROP NOT NULL;
DROP INDEX IF EXISTS idx_unique_runtime_architecture;
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_runtime_architecture ON runtime_architectures (runtime_framework_id, architecture_name,model_name);
