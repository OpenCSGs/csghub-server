SET statement_timeout = 0;

--bun:split

ALTER TABLE llm_configs
RENAME COLUMN official_name TO display_name;
