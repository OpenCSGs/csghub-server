SET statement_timeout = 0;

--bun:split

-- Create a unique index on provider and model_name to prevent duplicate entries
CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_configs_provider_model_name ON llm_configs (provider, model_name);
