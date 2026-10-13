SET statement_timeout = 0;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS balance_value_prev;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS token_plan_period_id;

--bun:split

ALTER TABLE account_statements DROP COLUMN IF EXISTS recorded_at_unix;

--bun:split

ALTER TABLE IF EXISTS account_statements_partitioned DROP COLUMN IF EXISTS balance_value_prev;

--bun:split

ALTER TABLE IF EXISTS account_statements_partitioned DROP COLUMN IF EXISTS token_plan_period_id;

--bun:split

ALTER TABLE IF EXISTS account_statements_partitioned DROP COLUMN IF EXISTS recorded_at_unix;
