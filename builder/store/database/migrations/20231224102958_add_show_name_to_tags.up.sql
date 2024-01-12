SET statement_timeout = 0;

--bun:split

ALTER TABLE tags ADD COLUMN IF NOT EXISTS show_name VARCHAR;

