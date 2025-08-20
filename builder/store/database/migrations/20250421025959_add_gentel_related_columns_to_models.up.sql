SET statement_timeout = 0;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS report_url VARCHAR;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS medium_risk_count INTEGER DEFAULT 0;

--bun:split

ALTER TABLE models ADD COLUMN IF NOT EXISTS high_risk_count INTEGER DEFAULT 0;
