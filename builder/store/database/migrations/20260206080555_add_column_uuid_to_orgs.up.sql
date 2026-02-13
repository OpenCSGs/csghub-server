SET statement_timeout = 0;

--bun:split

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

--bun:split

ALTER TABLE organizations ADD COLUMN IF NOT EXISTS uuid UUID DEFAULT gen_random_uuid();

--bun:split

UPDATE organizations SET uuid = gen_random_uuid() WHERE uuid IS NULL;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_uuid ON organizations(uuid);
