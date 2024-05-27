SET statement_timeout = 0;

--bun:split

ALTER TABLE cluster_infos DROP COLUMN IF EXISTS zone;

--bun:split

ALTER TABLE cluster_infos DROP COLUMN IF EXISTS provider;

--bun:split

ALTER TABLE cluster_infos DROP COLUMN IF EXISTS enable;
