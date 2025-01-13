SET statement_timeout = 0;

--bun:split

ALTER TABLE tag_categories ADD COLUMN IF NOT EXISTS show_name VARCHAR;

--bun:split

ALTER TABLE tag_categories ADD COLUMN IF NOT EXISTS enabled BOOLEAN DEFAULT true;
