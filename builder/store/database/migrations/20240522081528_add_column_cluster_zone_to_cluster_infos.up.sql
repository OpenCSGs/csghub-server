SET statement_timeout = 0;

--bun:split
ALTER TABLE cluster_infos ADD COLUMN IF NOT EXISTS zone VARCHAR;

--bun:split

ALTER TABLE cluster_infos ADD COLUMN IF NOT EXISTS provider VARCHAR;

--bun:split

ALTER TABLE cluster_infos ADD COLUMN IF NOT EXISTS enable BOOLEAN DEFAULT true;
