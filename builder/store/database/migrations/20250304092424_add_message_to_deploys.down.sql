SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS message;
ALTER TABLE deploys DROP COLUMN IF EXISTS reason;