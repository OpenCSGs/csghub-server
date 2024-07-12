SET statement_timeout = 0;

--bun:split

ALTER TABLE deploys ALTER COLUMN cost_per_hour TYPE double precision;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS casdoor_uuid VARCHAR;

--bun:split

ALTER TABLE space_resources ADD COLUMN IF NOT EXISTS cost_per_hour double precision;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS cost_per_hour double precision;

--bun:split

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS sku VARCHAR;

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS sku VARCHAR;
