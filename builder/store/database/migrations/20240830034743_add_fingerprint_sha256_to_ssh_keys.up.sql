SET statement_timeout = 0;

--bun:split

ALTER TABLE ssh_keys ADD COLUMN IF NOT EXISTS fingerprint_sha256 varchar(64);

