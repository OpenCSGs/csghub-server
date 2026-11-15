SET statement_timeout = 0;

--bun:split

-- External registrations only describe agents that live outside the platform.
-- Once the provenance columns are gone these rows are indistinguishable from
-- platform runtime instances, so remove them instead of leaving catalog entries
-- that quota checks, listing, and status lookups would treat as sandbox-backed.
DELETE FROM agent_instances WHERE source = 'external';

--bun:split

DROP INDEX IF EXISTS idx_agent_instances_relay_agent_id_unique;

ALTER TABLE agent_instances
    DROP CONSTRAINT IF EXISTS agent_instances_source_check;

--bun:split

ALTER TABLE agent_instances
    DROP COLUMN IF EXISTS relay_agent_id,
    DROP COLUMN IF EXISTS source;
