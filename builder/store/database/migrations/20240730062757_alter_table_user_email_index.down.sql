SET statement_timeout = 0;

--bun:split

-- alter table users allow email to be null
ALTER TABLE users ALTER COLUMN email SET NOT NULL;