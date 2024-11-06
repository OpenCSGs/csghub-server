SET statement_timeout = 0;

--bun:split

-- add new column sensitive_check_status to table repositories
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS sensitive_check_status int default 0;
