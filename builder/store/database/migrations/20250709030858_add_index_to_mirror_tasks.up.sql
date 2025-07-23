SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_mirror_tasks_mirror_id ON mirror_tasks (mirror_id);

