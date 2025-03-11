SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS message VARCHAR;
ALTER TABLE deploys ADD COLUMN IF NOT EXISTS reason VARCHAR;
