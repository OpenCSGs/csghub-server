SET statement_timeout = 0;

--bun:split

ALTER TABLE llm_configs
    ADD COLUMN IF NOT EXISTS model_size_b DOUBLE PRECISION NOT NULL DEFAULT 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_llm_configs_model_size_b ON llm_configs (model_size_b);
