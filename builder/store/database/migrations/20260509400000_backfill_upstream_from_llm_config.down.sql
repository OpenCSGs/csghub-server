SET statement_timeout = 0;

--bun:split

-- Data backfill is intentionally not reversible.
-- The upstreams JSONB fields were populated from llm_configs columns
-- that remain unchanged. No schema changes to revert.
