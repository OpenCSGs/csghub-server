SET statement_timeout = 0;

--bun:split

ALTER table users DROP COLUMN IF EXISTS "casdoor_uuid" ;

