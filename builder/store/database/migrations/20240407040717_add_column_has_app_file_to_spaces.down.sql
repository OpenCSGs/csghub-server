SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces  drop column IF EXISTS  has_app_file;

