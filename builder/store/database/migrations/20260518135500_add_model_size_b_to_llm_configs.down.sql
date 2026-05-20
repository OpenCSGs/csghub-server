SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_llm_configs_model_size_b;

--bun:split

ALTER TABLE llm_configs DROP COLUMN IF EXISTS model_size_b;
