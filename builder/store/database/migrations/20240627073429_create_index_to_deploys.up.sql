SET statement_timeout = 0;

--bun:split

CREATE INDEX IF NOT EXISTS idx_deploys_user_id_type ON deploys (user_id,type);