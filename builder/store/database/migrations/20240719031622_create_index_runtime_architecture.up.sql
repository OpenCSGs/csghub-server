SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_runtime_architecture ON runtime_architectures (runtime_framework_id, architecture_name);

