SET statement_timeout = 0;

--bun:split

-- Drop the per-upstream limit_policy column: the usage limiter
-- (UsageLimiter / CommitUsageLimitFromUsage) has been removed and the
-- per-upstream capacity_policy column replaces quota enforcement.
ALTER TABLE ai_gateway_upstreams DROP COLUMN IF EXISTS limit_policy;
