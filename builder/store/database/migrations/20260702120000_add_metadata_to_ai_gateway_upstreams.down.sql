SET statement_timeout = 0;
--bun:split
ALTER TABLE ai_gateway_upstreams DROP COLUMN IF EXISTS metadata;
