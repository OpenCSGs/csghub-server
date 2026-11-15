SET statement_timeout = 0;

--bun:split

ALTER TABLE account_activities DROP COLUMN IF EXISTS voucher_token_value;

--bun:split

ALTER TABLE account_activities DROP COLUMN IF EXISTS voucher_compute_value;

--bun:split

ALTER TABLE account_activities DROP COLUMN IF EXISTS valid_duration_month;

--bun:split

UPDATE account_activities SET voucher_value=1000 WHERE act_id = 1005;
