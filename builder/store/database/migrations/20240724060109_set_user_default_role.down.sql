SET statement_timeout = 0;

--bun:split

ALTER TABLE users ALTER COLUMN role_mask SET DEFAULT null;
