SET statement_timeout = 0;

--bun:split

ALTER TABLE organizations ADD COLUMN IF NOT EXISTS industry VARCHAR;

