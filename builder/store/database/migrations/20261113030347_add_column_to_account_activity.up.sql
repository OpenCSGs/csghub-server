SET statement_timeout = 0;

--bun:split

ALTER TABLE account_activities Add COLUMN IF NOT EXISTS voucher_token_value bigint DEFAULT 0;

--bun:split

ALTER TABLE account_activities Add COLUMN IF NOT EXISTS voucher_compute_value bigint DEFAULT 0;

--bun:split

ALTER TABLE account_activities Add COLUMN IF NOT EXISTS valid_duration_month bigint DEFAULT 0;

--bun:split

UPDATE account_activities SET voucher_value=3000, voucher_token_value=3000, voucher_compute_value=0, valid_duration_month=1 WHERE act_id = 1005;
