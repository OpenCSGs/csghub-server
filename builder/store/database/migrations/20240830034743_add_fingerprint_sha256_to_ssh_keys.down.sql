
SET statement_timeout = 0;

--bun:split

ALTER TABLE ssh_keys DROP COLUMN IF EXISTS fingerprint_sha256;
