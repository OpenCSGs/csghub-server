SET statement_timeout = 0;

--bun:split

ALTER TABLE organizations DROP COLUMN IF EXISTS industry;
