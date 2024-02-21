SET statement_timeout = 0;

--bun:split

ALTER TABLE repository_downloads DROP COLUMN IF EXISTS click_download_count;

--bun:split

ALTER TABLE repository_downloads RENAME COLUMN IF EXISTS clone_count TO count;
