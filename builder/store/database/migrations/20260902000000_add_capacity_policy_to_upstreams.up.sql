SET statement_timeout = 0;

--bun:split

-- CapacityPolicy adds per-upstream capacity limits (concurrency, queue, TPM,
-- RPM). The existing per-user limit_policy column is retained: the UsageLimiter
-- keeps enforcing it, and the upcoming capacity tracker will follow the same
-- check/commit pattern.
ALTER TABLE ai_gateway_upstreams ADD COLUMN IF NOT EXISTS capacity_policy jsonb DEFAULT NULL;
