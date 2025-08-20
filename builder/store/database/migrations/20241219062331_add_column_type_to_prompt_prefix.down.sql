SET statement_timeout = 0;

--bun:split

ALTER TABLE prompt_prefixes DROP COLUMN IF EXISTS kind;
