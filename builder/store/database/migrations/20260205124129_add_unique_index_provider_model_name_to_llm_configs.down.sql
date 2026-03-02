SET statement_timeout = 0;

--bun:split

-- Drop the unique index on provider and model_name
DROP INDEX IF EXISTS idx_llm_configs_provider_model_name;
