SET statement_timeout = 0;

--bun:split

-- not needed anymore, as we have unique index on email
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;

-- alter table users allow email to be null
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;