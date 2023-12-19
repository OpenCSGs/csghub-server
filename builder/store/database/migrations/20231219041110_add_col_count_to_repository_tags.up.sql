SET statement_timeout = 0;

--bun:split

ALTER TABLE repository_tags ADD COLUMN IF NOT EXIST count INT DEFAULT 1;

--bun:split
