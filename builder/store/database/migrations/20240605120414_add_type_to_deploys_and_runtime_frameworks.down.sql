SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS type;
--bun:split

ALTER TABLE runtime_frameworks DROP COLUMN IF EXISTS type;
