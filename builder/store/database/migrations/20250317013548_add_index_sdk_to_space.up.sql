SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_spaces_sdk ON spaces (sdk);
