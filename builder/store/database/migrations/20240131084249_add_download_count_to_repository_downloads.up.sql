SET statement_timeout = 0;

--bun:split

ALTER TABLE repository_downloads ADD COLUMN IF NOT EXISTS click_download_count INT DEFAULT 0;

--bun:split

ALTER TABLE repository_downloads RENAME COLUMN IF EXISTS count TO clone_count;
