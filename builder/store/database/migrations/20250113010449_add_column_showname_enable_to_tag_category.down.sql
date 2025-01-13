SET statement_timeout = 0;

--bun:split

ALTER TABLE tag_categories DROP COLUMN IF EXISTS show_name;

--bun:split

ALTER TABLE tag_categories DROP COLUMN IF EXISTS enabled;
