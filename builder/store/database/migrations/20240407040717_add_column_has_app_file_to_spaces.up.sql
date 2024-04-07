SET statement_timeout = 0;

--bun:split
ALTER TABLE spaces  add column IF NOT EXISTS  has_app_file bool NULL;

