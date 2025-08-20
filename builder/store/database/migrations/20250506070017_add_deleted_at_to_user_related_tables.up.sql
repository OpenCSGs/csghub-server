SET statement_timeout = 0;

--bun:split

ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE users ADD COLUMN IF NOT EXISTS retain_data VARCHAR;

--bun:split

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE collections ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE lfs_locks ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE members ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE namespaces ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE ssh_keys ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE user_likes ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE discussions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE prompt_conversations ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

--bun:split

ALTER TABLE comments ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;