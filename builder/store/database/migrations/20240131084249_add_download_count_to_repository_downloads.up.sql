SET statement_timeout = 0;

--bun:split

ALTER TABLE repository_downloads ADD COLUMN IF NOT EXISTS click_download_count INT DEFAULT 0;

--bun:split

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns 
        WHERE table_name='repository_downloads' 
        AND column_name='count'
    ) THEN
        EXECUTE 'ALTER TABLE repository_downloads RENAME COLUMN count TO clone_count;';
    END IF;
END $$;
