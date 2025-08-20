SET statement_timeout = 0;

--bun:split

ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE users DROP COLUMN IF EXISTS retain_data;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE access_tokens DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE collections DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE lfs_locks DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE members DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE namespaces DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE ssh_keys DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE user_likes DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE discussions DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE prompt_conversations DROP COLUMN IF EXISTS deleted_at;

--bun:split

ALTER TABLE comments DROP COLUMN IF EXISTS deleted_at;