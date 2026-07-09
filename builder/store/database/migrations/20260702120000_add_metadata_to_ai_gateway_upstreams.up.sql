SET statement_timeout = 0;
--bun:split
ALTER TABLE ai_gateway_upstreams
ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT NULL;
