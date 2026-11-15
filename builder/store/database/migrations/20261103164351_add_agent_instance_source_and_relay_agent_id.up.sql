SET statement_timeout = 0;

--bun:split

-- source records where an instance's runtime lives: platform-hosted agents are
-- provisioned by StarHub, while external agents run outside the platform (a
-- user's local machine, their own server, or third-party infrastructure) and are
-- reachable through CSGRelay. Existing rows are platform-hosted.
ALTER TABLE agent_instances
    ADD COLUMN IF NOT EXISTS source varchar(32) NOT NULL DEFAULT 'platform';

ALTER TABLE agent_instances
    ADD CONSTRAINT agent_instances_source_check CHECK (source IN ('platform', 'external'));

--bun:split

-- relay_agent_id is the stable CSGRelay agent id reported by a CSGClaw worker.
-- It is NULL until registration and is written alongside the metadata
-- registration so the catalog can filter registered instances across both
-- sources.
ALTER TABLE agent_instances
    ADD COLUMN IF NOT EXISTS relay_agent_id varchar(64);

--bun:split

-- A relay agent id identifies exactly one live instance.
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_instances_relay_agent_id_unique
    ON agent_instances (relay_agent_id)
    WHERE relay_agent_id IS NOT NULL AND relay_agent_id <> '' AND deleted_at IS NULL;
