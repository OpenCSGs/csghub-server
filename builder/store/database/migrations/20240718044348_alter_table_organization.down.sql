SET statement_timeout = 0;

--bun:split

ALTER TABLE organizations DROP COLUMN IF EXISTS homepage;
ALTER TABLE organizations DROP COLUMN IF EXISTS logo;
ALTER TABLE organizations DROP COLUMN IF EXISTS org_type;
ALTER TABLE organizations DROP COLUMN IF EXISTS verified;
