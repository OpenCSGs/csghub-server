SET statement_timeout = 0;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS report_url;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS medium_risk_count;

--bun:split

ALTER TABLE models DROP COLUMN IF EXISTS high_risk_count;
