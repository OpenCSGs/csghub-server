SET statement_timeout = 0;

--bun:split

ALTER TABLE tags DROP COLUMN IF EXISTS i18n_key;
