SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_account_presents_activityid_status_expireat;

--bun:split

ALTER TABLE account_presents DROP COLUMN IF EXISTS status;

--bun:split

ALTER TABLE account_presents DROP COLUMN IF EXISTS expire_at;

--bun:split

ALTER TABLE account_presents DROP COLUMN IF EXISTS participant_uuid;