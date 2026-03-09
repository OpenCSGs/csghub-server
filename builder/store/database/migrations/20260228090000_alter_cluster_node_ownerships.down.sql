SET
statement_timeout = 0;

--bun:split

ALTER TABLE cluster_node_ownerships
    ADD COLUMN IF NOT EXISTS user_uuid varchar (64);

--bun:split

ALTER TABLE cluster_node_ownerships
    ADD COLUMN IF NOT EXISTS org_uuid varchar (64);

--bun:split

DROP INDEX IF EXISTS idx_cluster_node_ownership_namespace;

--bun:split

ALTER TABLE cluster_node_ownerships DROP COLUMN IF EXISTS namespace;
