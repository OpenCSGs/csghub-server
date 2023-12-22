SET statement_timeout = 0;

--bun:split

ALTER TABLE tags ADD COLUMN IF NOT EXISTS built_in BOOLEAN DEFAULT false;

--bun:split

DROP INDEX IF EXISTS idx_tags_name_scope;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_name_category_scope ON tags(name, category, scope);