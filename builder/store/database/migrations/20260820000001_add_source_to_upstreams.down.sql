SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_ai_gateway_upstreams_source_source_id;

--bun:split

ALTER TABLE ai_gateway_upstreams DROP COLUMN IF EXISTS source_id;

--bun:split

ALTER TABLE ai_gateway_upstreams DROP COLUMN IF EXISTS source;
