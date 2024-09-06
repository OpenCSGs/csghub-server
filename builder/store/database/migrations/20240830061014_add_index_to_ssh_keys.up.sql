SET statement_timeout = 0;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_ssk_keys_fingerprint_sha256_unique ON ssh_keys (fingerprint_sha256);

