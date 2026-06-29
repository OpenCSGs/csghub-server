SET statement_timeout = 0;

--bun:split

ALTER TABLE metadata
    ADD COLUMN IF NOT EXISTS pd_recommendation JSONB;

ALTER TABLE metadata
    ADD COLUMN IF NOT EXISTS model_arch_type VARCHAR(20) NOT NULL DEFAULT 'dense';