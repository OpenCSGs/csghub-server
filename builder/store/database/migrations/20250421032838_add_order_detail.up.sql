SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS order_detail_id BIGINT;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS order_detail_id BIGINT;
