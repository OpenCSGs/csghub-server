SET statement_timeout = 0;

--bun:split

ALTER TABLE git_server_access_tokens ADD COLUMN IF NOT EXISTS server_type VARCHAR(255) NOT NULL DEFAULT 'git';
