SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS engine_args;