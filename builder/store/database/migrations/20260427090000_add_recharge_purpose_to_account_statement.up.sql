ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS purpose VARCHAR;

--bun:split

ALTER TABLE account_statements ADD COLUMN IF NOT EXISTS purpose_desc VARCHAR;
