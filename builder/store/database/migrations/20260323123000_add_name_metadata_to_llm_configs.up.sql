SET statement_timeout = 0;

--bun:split

ALTER TABLE llm_configs ADD COLUMN IF NOT EXISTS display_name TEXT DEFAULT '';

--bun:split

ALTER TABLE llm_configs ADD COLUMN IF NOT EXISTS metadata JSONB;
