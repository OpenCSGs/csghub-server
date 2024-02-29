SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS likes;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS download_count;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS nickname;