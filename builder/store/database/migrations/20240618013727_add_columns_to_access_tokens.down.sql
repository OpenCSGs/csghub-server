SET statement_timeout = 0;

--bun:split

ALTER TABLE access_tokens DROP COLUMN IF EXISTS expired_at;

--bun:split

ALTER TABLE access_tokens DROP COLUMN IF EXISTS is_active;

--bun:split

ALTER TABLE access_tokens DROP COLUMN IF EXISTS app;

--bun:split

ALTER TABLE access_tokens DROP COLUMN IF EXISTS permission;