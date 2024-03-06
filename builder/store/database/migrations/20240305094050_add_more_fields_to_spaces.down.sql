SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS sdk_id;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS resource_id;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS cover_image_url;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS resource_id;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS sdk VARCHAR;