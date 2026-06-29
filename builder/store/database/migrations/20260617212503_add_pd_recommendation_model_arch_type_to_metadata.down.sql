SET statement_timeout = 0;

--bun:split

ALTER TABLE metadata DROP COLUMN IF EXISTS pd_recommendation;

ALTER TABLE metadata DROP COLUMN IF EXISTS model_arch_type;