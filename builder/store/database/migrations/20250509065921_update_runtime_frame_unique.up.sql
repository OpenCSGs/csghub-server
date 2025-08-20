SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_unique_frame_driver_compute;
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_frame_driver_compute ON runtime_frameworks (frame_name, compute_type, frame_image);