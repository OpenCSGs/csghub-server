SET statement_timeout = 0;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS name VARCHAR;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS description VARCHAR;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS path VARCHAR;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS git_path VARCHAR;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS user_id INT;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS private BOOLEAN;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS likes INT;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS downloads INT;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS url_slug VARCHAR;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS name VARCHAR;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS description VARCHAR;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS path VARCHAR;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS git_path VARCHAR;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS user_id INT;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS private BOOLEAN;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS likes INT;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS downloads INT;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS url_slug VARCHAR;