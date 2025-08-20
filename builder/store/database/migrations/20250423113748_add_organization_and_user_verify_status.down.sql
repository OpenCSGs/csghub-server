SET statement_timeout = 0;

--bun:split

ALTER TABLE "users" DROP COLUMN IF EXISTS "verify_status";

--bun:split

ALTER TABLE "organizations" DROP COLUMN IF EXISTS "verify_status";

