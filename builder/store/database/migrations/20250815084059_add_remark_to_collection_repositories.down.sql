SET statement_timeout = 0;

--bun:split

ALTER TABLE collection_repositories DROP COLUMN IF EXISTS remark;

ALTER TABLE collection_repositories DROP COLUMN IF EXISTS created_at;

ALTER TABLE collection_repositories DROP COLUMN IF EXISTS updated_at;
