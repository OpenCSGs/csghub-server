SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_organizations_uuid;

--bun:split

ALTER TABLE organizations DROP COLUMN IF EXISTS uuid;
