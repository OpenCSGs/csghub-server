SET statement_timeout = 0;

--bun:split

ALTER TABLE account_token_plan_instance_periods ADD COLUMN IF NOT EXISTS cost_amount DOUBLE PRECISION NOT NULL DEFAULT 0;
