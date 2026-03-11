SET
statement_timeout = 0;

--bun:split

ALTER TABLE cluster_node_ownerships
    ADD COLUMN IF NOT EXISTS namespace varchar NOT NULL DEFAULT '';

--bun:split

CREATE INDEX IF NOT EXISTS idx_cluster_node_ownership_namespace ON cluster_node_ownerships (namespace);

--bun:split

ALTER TABLE cluster_node_ownerships DROP COLUMN IF EXISTS user_uuid;

--bun:split

ALTER TABLE cluster_node_ownerships DROP COLUMN IF EXISTS org_uuid;
