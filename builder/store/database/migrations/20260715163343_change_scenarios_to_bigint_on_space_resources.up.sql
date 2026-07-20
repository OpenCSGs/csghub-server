SET statement_timeout = 0;

--bun:split

-- scenarios was a freshly added text[] column with no production data yet, so
-- drop and re-add as a bigint bitmask instead of an in-place type conversion.
-- Each scenario occupies one bit (see common/types/space_resource.go scenarioBit):
--   bit7 = sandbox (128). 0 = supports nothing, -1 = supports all scenarios.
ALTER TABLE space_resources DROP COLUMN IF EXISTS scenarios;

--bun:split

-- The column default is 0 (supports nothing), matching the Go struct's
-- `bun:",notnull,default:0"` and the API/Create semantics: a resource created
-- without scenarios is invisible to scenario-filtered queries. -129 is NOT the
-- column default — it is only used below to backfill EXISTING rows so the
-- pre-bitmap resources (which supported everything except sandbox) stay visible
-- after the column type change.
ALTER TABLE space_resources ADD COLUMN scenarios bigint NOT NULL DEFAULT 0;

--bun:split

-- Backfill existing rows: set them to "all scenarios except sandbox":
-- bitmask -129 == ~128, i.e. all 64 bits set except bit7 (sandbox). Sandbox is
-- intentionally left unset here and is handled manually per resource.
--   -1 (all bits) & ~128 = -129
-- This only touches rows that are still 0 (the freshly added default); rows
-- configured later (including sandbox) are left intact.
-- See common/types/space_resource.go scenarioBit for the bit assignment.
UPDATE space_resources SET scenarios = -129 WHERE scenarios = 0;

