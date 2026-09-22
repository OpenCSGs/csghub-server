SET statement_timeout = 0;

--bun:split

DROP TABLE IF EXISTS account_token_report_daily_summaries;

--bun:split

ALTER TABLE account_statements
	DROP COLUMN IF EXISTS cost_amount,
	DROP COLUMN IF EXISTS cost_prompt_sku_id,
	DROP COLUMN IF EXISTS cost_completion_sku_id,
	DROP COLUMN IF EXISTS provider,
	DROP COLUMN IF EXISTS upstream_id;

--bun:split

ALTER TABLE IF EXISTS account_statements_partitioned
	DROP COLUMN IF EXISTS cost_amount,
	DROP COLUMN IF EXISTS cost_prompt_sku_id,
	DROP COLUMN IF EXISTS cost_completion_sku_id,
	DROP COLUMN IF EXISTS provider,
	DROP COLUMN IF EXISTS upstream_id;
