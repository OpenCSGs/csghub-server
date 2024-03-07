SET statement_timeout = 0;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS template;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS env;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS hardware;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS cover_image_url;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS secrets;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS sdk_version;