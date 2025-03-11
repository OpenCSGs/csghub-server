SET statement_timeout = 0;

--bun:split

UPDATE users SET uuid = gen_random_uuid() WHERE uuid = '' or uuid IS NULL;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_uuid ON users (uuid);