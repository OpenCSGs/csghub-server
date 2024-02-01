SET statement_timeout = 0;

--bun:split

ALTER TABLE repository_downloads DROP COLUMN click_download_count;

--bun:split

ALTER TABLE repository_downloads RENAME COLUMN clone_count TO count;
