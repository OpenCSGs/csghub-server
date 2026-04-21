SET statement_timeout = 0;

--bun:split

ALTER TABLE account_presents ADD COLUMN IF NOT EXISTS participant_uuid VARCHAR(36) DEFAULT NULL;

--bun:split

ALTER TABLE account_presents ADD COLUMN IF NOT EXISTS expire_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;

--bun:split

ALTER TABLE account_presents ADD COLUMN IF NOT EXISTS status int DEFAULT 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_account_presents_activityid_status_expireat 
ON account_presents (activity_id, status, expire_at)
WHERE expire_at IS NOT NULL;