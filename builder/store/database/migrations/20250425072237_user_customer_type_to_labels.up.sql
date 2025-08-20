SET
statement_timeout = 0;

--bun:split

ALTER TABLE users
DROP
COLUMN IF EXISTS customer_type;

ALTER TABLE users
    ADD COLUMN labels JSONB DEFAULT '[]' NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_labels ON users USING GIN (labels);


