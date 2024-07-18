SET statement_timeout = 0;

--bun:split

ALTER TABLE collection_repositories DROP CONSTRAINT IF EXISTS fk_collection;

--bun:split

ALTER TABLE collection_repositories DROP CONSTRAINT IF EXISTS fk_repository;

--bun:split

ALTER TABLE user_likes DROP COLUMN IF EXISTS collection_id;
