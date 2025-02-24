SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS variables;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS variables;