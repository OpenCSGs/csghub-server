SET statement_timeout = 0;

--bun:split

ALTER TABLE tags ADD COLUMN IF NOT EXISTS i18n_key VARCHAR;

UPDATE tags
SET i18n_key = CASE
    WHEN show_name ~ '[\u4e00-\u9fa5]$' OR show_name = '' THEN name
    ELSE show_name
END
WHERE built_in is true;