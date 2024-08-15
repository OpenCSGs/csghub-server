SET statement_timeout = 0;

--bun:split

ALTER TABLE cluster_infos DROP COLUMN IF EXISTS storage_class;
