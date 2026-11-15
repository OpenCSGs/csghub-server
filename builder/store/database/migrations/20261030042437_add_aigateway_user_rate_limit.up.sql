SET statement_timeout = 0;

--bun:split

-- Per-user rate limit policy (RPM/TPM) on the model level: a model with an
-- enabled policy overrides the gateway-global one (configured statically in
-- the AIGateway.UserRateLimit config section, not in the database).
ALTER TABLE llm_configs ADD COLUMN IF NOT EXISTS rate_limit jsonb;
