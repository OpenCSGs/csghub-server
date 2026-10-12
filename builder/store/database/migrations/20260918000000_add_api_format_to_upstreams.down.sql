SET statement_timeout = 0;

--bun:split

ALTER TABLE ai_gateway_upstreams DROP COLUMN IF EXISTS supported_endpoints;
ALTER TABLE ai_gateway_upstreams DROP COLUMN IF EXISTS is_full_url;
