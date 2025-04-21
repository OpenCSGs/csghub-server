SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS order_detail_id;

--bun:split

ALTER TABLE spaces DROP COLUMN IF EXISTS order_detail_id;
