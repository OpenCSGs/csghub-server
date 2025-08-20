SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS order_detail_id BIGINT;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS order_detail_id BIGINT;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS cost_per_hour;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS cost_per_hour;

--bun:split

ALTER TABLE space_resources DROP COLUMN IF EXISTS cost_per_hour;

