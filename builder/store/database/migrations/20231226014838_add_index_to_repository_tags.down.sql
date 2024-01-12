SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_repository_tags_repository_id;

--bun:split

DROP INDEX IF EXISTS idx_repository_tags_tag_id;

--bun:split

DROP INDEX IF EXISTS idx_repository_tags_repository_id_tag_id;

