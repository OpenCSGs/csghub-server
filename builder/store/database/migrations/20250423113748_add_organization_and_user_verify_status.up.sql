SET
statement_timeout = 0;

--bun:split
ALTER TABLE "users"
    ADD COLUMN IF NOT EXISTS "verify_status" VARCHAR NOT NULL DEFAULT 'none';

ALTER TABLE "organizations"
    ADD COLUMN IF NOT EXISTS "verify_status" VARCHAR NOT NULL DEFAULT 'none';