SET statement_timeout = 0;

--bun:split

-- add bool column can_change_user_name to table users
ALTER TABLE users ADD COLUMN IF NOT EXISTS can_change_user_name boolean NOT NULL DEFAULT false