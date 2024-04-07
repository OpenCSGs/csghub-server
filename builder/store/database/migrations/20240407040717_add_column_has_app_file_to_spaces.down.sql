SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces  DROP COLUMN IF EXISTS  has_app_file;

--bun:split
DROP  INDEX IF  EXISTS idx_deploys_space_id_created_at;
