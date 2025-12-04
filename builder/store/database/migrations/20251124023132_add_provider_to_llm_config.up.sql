SET statement_timeout = 0;

--bun:split

ALTER TABLE llm_configs ADD COLUMN IF NOT EXISTS provider TEXT DEFAULT '';
