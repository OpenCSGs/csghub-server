SET statement_timeout = 0;

--bun:split

ALTER TABLE ai_gateway_upstreams ALTER COLUMN health_check_enabled SET DEFAULT false;

--bun:split

ALTER TABLE ai_gateway_upstreams ALTER COLUMN circuit_breaker_enabled SET DEFAULT false;
