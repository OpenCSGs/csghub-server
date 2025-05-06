SET statement_timeout = 0;

--bun:split

ALTER TABLE repositories DROP COLUMN IF EXISTS star_count;
