SET statement_timeout = 0;

--bun:split

ALTER TABLE llm_configs DROP COLUMN IF EXISTS provider;
