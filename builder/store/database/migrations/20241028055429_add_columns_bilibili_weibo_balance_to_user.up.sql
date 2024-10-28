SET statement_timeout = 0;

--bun:split

ALTER TABLE users ADD COLUMN IF NOT EXISTS bilibili VARCHAR;

--bun:split

ALTER TABLE users ADD COLUMN IF NOT EXISTS weibo VARCHAR;

--bun:split

ALTER TABLE users ADD COLUMN IF NOT EXISTS balance INT default 30;
