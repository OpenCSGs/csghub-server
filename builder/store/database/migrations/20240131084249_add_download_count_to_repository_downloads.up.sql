SET statement_timeout = 0;

--bun:split

ALTER TABLE repository_downloads ADD click_download_count INT DEFAULT 0;

--bun:split

ALTER TABLE repository_downloads RENAME COLUMN count TO clone_count;
