SET statement_timeout = 0;

--bun:split
ALTER TABLE organization_verifies ADD COLUMN IF NOT EXISTS user_uuid TEXT;
