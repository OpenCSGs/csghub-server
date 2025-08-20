SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS order_detail_id;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS order_detail_id;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS cost_per_hour double precision;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS cost_per_hour double precision;

--bun:split

ALTER TABLE space_resources ADD COLUMN IF NOT EXISTS cost_per_hour double precision;
