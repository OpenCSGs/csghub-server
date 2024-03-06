SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS sdk_id INT NOT NULL;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS resource_id INT NOT NULL;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS cover_image_url VARCHAR;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS resource_id INT NOT NULL;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS sdk;