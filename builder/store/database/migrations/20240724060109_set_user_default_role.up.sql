SET statement_timeout = 0;

--bun:split

-- set default value of roles of table users to 'personal_user'
ALTER TABLE users ALTER COLUMN role_mask SET DEFAULT 'personal_user'