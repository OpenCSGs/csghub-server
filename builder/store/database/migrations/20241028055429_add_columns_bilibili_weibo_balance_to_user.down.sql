SET statement_timeout = 0;

--bun:split

ALTER TABLE users DROP COLUMN IF EXISTS bilibili;

--bun:split

ALTER TABLE users DROP COLUMN IF EXISTS weibo;

--bun:split

ALTER TABLE users DROP COLUMN IF EXISTS balance;
