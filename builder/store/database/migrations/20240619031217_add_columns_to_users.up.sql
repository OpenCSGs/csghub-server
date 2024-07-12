SET statement_timeout = 0;

--bun:split

ALTER table users ADD COLUMN IF NOT EXISTS "casdoor_uuid" VARCHAR;


