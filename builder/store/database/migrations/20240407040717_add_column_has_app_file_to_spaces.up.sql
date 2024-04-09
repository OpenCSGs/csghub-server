SET statement_timeout = 0;

--bun:split
ALTER TABLE spaces  ADD COLUMN IF NOT EXISTS  has_app_file bool NULL;

--bun:split
CREATE INDEX IF NOT EXISTS idx_deploys_space_id_created_at ON deploys (space_id,created_at);
