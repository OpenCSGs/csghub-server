SET statement_timeout = 0;

--bun:split

ALTER TABLE account_token_plan_instance_periods DROP COLUMN IF EXISTS cost_amount;
