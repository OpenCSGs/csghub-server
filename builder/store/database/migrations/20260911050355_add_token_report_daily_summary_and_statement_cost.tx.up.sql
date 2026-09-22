SET statement_timeout = 0;

--bun:split

-- Charge-time upstream procurement cost snapshot + provider/upstream attribution.
-- account_statements has a saas history-archive twin (account_statements_partitioned)
-- synced by a fail-loud trigger using SELECT NEW.*, so both tables must gain the
-- same columns in one transaction.

ALTER TABLE account_statements
	ADD COLUMN IF NOT EXISTS cost_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS cost_prompt_sku_id BIGINT,
	ADD COLUMN IF NOT EXISTS cost_completion_sku_id BIGINT,
	ADD COLUMN IF NOT EXISTS provider VARCHAR NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS upstream_id BIGINT NOT NULL DEFAULT 0;

--bun:split

ALTER TABLE IF EXISTS account_statements_partitioned
	ADD COLUMN IF NOT EXISTS cost_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS cost_prompt_sku_id BIGINT,
	ADD COLUMN IF NOT EXISTS cost_completion_sku_id BIGINT,
	ADD COLUMN IF NOT EXISTS provider VARCHAR NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS upstream_id BIGINT NOT NULL DEFAULT 0;

--bun:split

-- Daily fact rollup for the admin token report (issue #3404): frozen revenue
-- and cost aggregated from account_statements by the nightly Temporal job.
CREATE TABLE IF NOT EXISTS account_token_report_daily_summaries (
	id BIGSERIAL PRIMARY KEY,
	stat_date DATE NOT NULL,
	ns_uuid VARCHAR NOT NULL,
	token_id BIGINT NOT NULL DEFAULT 0,
	resource_id VARCHAR NOT NULL DEFAULT '',
	provider VARCHAR NOT NULL DEFAULT '',
	upstream_id BIGINT NOT NULL DEFAULT 0,
	scene INT NOT NULL,
	call_count BIGINT NOT NULL DEFAULT 0,
	prompt_token DOUBLE PRECISION NOT NULL DEFAULT 0,
	prompt_cached_token DOUBLE PRECISION NOT NULL DEFAULT 0,
	completion_token DOUBLE PRECISION NOT NULL DEFAULT 0,
	duration DOUBLE PRECISION NOT NULL DEFAULT 0,
	data_type VARCHAR(255) NOT NULL DEFAULT '',
	resolution VARCHAR(255) NOT NULL DEFAULT '',
	revenue_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
	cost_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
	updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp
);

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_token_report_daily_summary
	ON account_token_report_daily_summaries
	(stat_date, ns_uuid, token_id, resource_id, provider, upstream_id, scene, data_type, resolution);

--bun:split

CREATE INDEX IF NOT EXISTS idx_token_report_daily_summary_date
	ON account_token_report_daily_summaries (stat_date);

--bun:split

CREATE INDEX IF NOT EXISTS idx_token_report_daily_summary_date_provider
	ON account_token_report_daily_summaries (stat_date, provider);

--bun:split

CREATE INDEX IF NOT EXISTS idx_token_report_daily_summary_date_ns
	ON account_token_report_daily_summaries (stat_date, ns_uuid);
