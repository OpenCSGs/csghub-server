SET statement_timeout = 0;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS name;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS description;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS path;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS git_path;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS user_id;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS private;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS likes;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS downloads;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS url_slug;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS name;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS description;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS path;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS git_path;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS user_id;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS private;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS likes;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS downloads;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS url_slug;