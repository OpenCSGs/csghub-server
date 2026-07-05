SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS status_update_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;

--bun:split

UPDATE deploys SET status_update_at = updated_at;
