SET statement_timeout = 0;

--bun:split

ALTER TABLE cluster_infos ADD COLUMN IF NOT EXISTS resource_status VARCHAR;

