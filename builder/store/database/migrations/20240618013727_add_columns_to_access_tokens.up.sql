SET statement_timeout = 0;

--bun:split

ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS expired_at TIMESTAMP;

--bun:split

ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS is_active BOOLEAN default true;

--bun:split

ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS app VARCHAR default 'git';

--bun:split

ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS permission VARCHAR;

