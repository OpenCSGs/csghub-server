SET statement_timeout = 0;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS type INT DEFAULT 1;

--bun:split

CREATE INDEX IF NOT EXISTS idx_datasets_type ON datasets (type);
