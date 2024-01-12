SET statement_timeout = 0;

--bun:split

ALTER TABLE tags DROP COLUMN IF EXISTS built_in;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_name_scope ON tags(name, scope);

--bun:split

DROP INDEX IF EXISTS idx_tags_name_category_scope;