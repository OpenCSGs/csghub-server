SET statement_timeout = 0;

--bun:split

ALTER TABLE cluster_infos DROP COLUMN endpoint;

ALTER TABLE cluster_infos DROP COLUMN status;

ALTER TABLE cluster_infos DROP COLUMN mode;

ALTER TABLE cluster_infos DROP COLUMN created_at;

ALTER TABLE cluster_infos DROP COLUMN updated_at;