SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_deploys_status ON deploys (status);

