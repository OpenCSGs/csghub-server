SET statement_timeout = 0;

--bun:split

ALTER TABLE repository_downloads ADD COLUMN IF NOT EXISTS click_download_count INT DEFAULT 0;

--bun:split

CREATE OR REPLACE FUNCTION rename_column_if_exists(
    target_table TEXT,
    old_column_name TEXT,
    new_column_name TEXT
)
RETURNS void AS
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = target_table AND column_name = old_column_name
    )
    THEN
        EXECUTE 'ALTER TABLE ' || quote_ident(target_table) ||
                ' RENAME COLUMN ' || quote_ident(old_column_name) ||
                ' TO ' || quote_ident(new_column_name) || ';';
    ELSE
        RAISE NOTICE 'Column "%" does not exist in table "%".', old_column_name, target_table;
    END IF;
END;
$$
LANGUAGE plpgsql;

SELECT rename_column_if_exists('repository_downloads', 'count', 'clone_count');
