SET statement_timeout = 0;

--bun:split
ALTER TABLE runtime_frameworks ADD COLUMN IF NOT EXISTS frame_npu_image VARCHAR;

