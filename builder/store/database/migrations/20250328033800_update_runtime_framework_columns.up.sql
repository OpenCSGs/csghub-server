SET statement_timeout = 0;

--bun:split

ALTER TABLE runtime_frameworks ALTER COLUMN frame_image DROP NOT NULL;

--bun:split

ALTER TABLE runtime_frameworks ALTER COLUMN frame_cpu_image DROP NOT NULL;

--bun:split

ALTER TABLE runtime_frameworks ALTER COLUMN engine_args DROP NOT NULL;
