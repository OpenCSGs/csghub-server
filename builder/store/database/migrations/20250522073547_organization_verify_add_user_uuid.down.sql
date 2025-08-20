SET statement_timeout = 0;

--bun:split
ALTER TABLE organization_verifies DROP COLUMN IF EXISTS user_uuid;