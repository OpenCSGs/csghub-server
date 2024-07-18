SET statement_timeout = 0;

--bun:split

ALTER TABLE collection_repositories ADD CONSTRAINT fk_collection FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE CASCADE;

--bun:split
ALTER TABLE collection_repositories ADD CONSTRAINT fk_repository FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE;

--bun:split

ALTER TABLE user_likes ADD COLUMN IF NOT EXISTS collection_id bigint;
