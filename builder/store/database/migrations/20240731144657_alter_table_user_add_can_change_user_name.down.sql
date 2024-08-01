SET statement_timeout = 0;

--bun:split

ALTER TABLE users DROP COLUMN IF EXISTS can_change_user_name;