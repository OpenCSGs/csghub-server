SET statement_timeout = 0;

--bun:split

ALTER TABLE ai_gateway_upstreams
    ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 0;
