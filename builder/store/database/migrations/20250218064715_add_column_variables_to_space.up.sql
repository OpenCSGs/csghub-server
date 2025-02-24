SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS variables VARCHAR;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS variables VARCHAR;
