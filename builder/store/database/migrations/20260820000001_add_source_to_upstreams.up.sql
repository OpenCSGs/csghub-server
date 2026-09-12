SET statement_timeout = 0;

--bun:split

ALTER TABLE ai_gateway_upstreams ADD COLUMN IF NOT EXISTS source varchar(20) NOT NULL DEFAULT 'external';

--bun:split

ALTER TABLE ai_gateway_upstreams ADD COLUMN IF NOT EXISTS source_id bigint NOT NULL DEFAULT 0;

--bun:split

-- UNIQUE partial index: enforces one csghub upstream per (source, source_id) at the DB
-- level, preventing duplicate rows from concurrent UpsertInternalDeployTarget calls
-- (which use a read-then-write lookup by source + source_id).
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_gateway_upstreams_source_source_id
    ON ai_gateway_upstreams (source, source_id) WHERE source = 'csghub';
