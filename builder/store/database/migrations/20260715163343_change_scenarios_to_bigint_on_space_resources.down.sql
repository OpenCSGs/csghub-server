SET statement_timeout = 0;

--bun:split

-- Roll back the scenarios column from bigint bitmask to text[]. This restores
-- the schema the pre-bitmap code expects (Scenarios []string with bun:",array").
-- The bigint bitmask data is discarded — acceptable on rollback since the goal is
-- to return to the old behavior, not preserve new-style data. Existing resources
-- get an empty text[] (supports nothing) until re-tagged.
ALTER TABLE space_resources DROP COLUMN IF EXISTS scenarios;

--bun:split

ALTER TABLE space_resources ADD COLUMN scenarios text[] NOT NULL DEFAULT '{}';
