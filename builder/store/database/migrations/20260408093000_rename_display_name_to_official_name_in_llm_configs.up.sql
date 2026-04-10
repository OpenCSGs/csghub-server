SET statement_timeout = 0;

--bun:split

ALTER TABLE llm_configs
RENAME COLUMN display_name TO official_name;
