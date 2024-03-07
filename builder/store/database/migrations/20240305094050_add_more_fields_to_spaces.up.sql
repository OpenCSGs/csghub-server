SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS template VARCHAR;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS env VARCHAR(2048);

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS hardware VARCHAR(2048);

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS cover_image_url VARCHAR;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS secrets VARCHAR(2048);

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS sdk_version VARCHAR(2048);