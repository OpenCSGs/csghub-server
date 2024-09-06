SET statement_timeout = 0;

--bun:split

ALTER TABLE mirrors ADD COLUMN IF NOT EXISTS mirror_priority INT default 0;

