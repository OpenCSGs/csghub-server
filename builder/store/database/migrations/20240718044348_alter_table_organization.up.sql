SET statement_timeout = 0;

--bun:split

ALTER TABLE organizations ADD COLUMN IF NOT EXISTS homepage VARCHAR;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS logo VARCHAR;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS org_type VARCHAR;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS verified BOOLEAN;
