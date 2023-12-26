SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_repository_tags_repository_id ON repository_tags(repository_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_repository_tags_tag_id ON repository_tags(tag_id);

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_repository_tags_repository_id_tag_id ON repository_tags(repository_id, tag_id);