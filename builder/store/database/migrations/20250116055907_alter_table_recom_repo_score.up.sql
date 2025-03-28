SET statement_timeout = 0;

--bun:split

-- change primary key to id
ALTER TABLE recom_repo_scores DROP CONSTRAINT IF EXISTS recom_repo_scores_pkey;
ALTER TABLE recom_repo_scores ADD COLUMN IF NOT EXISTS id SERIAL PRIMARY KEY;

-- add cloumn weight_name, not null, default 'total'
ALTER TABLE recom_repo_scores ADD COLUMN IF NOT EXISTS weight_name VARCHAR(255)  NOT NULL DEFAULT 'total';

-- unique index (repository_id, weight_name)
CREATE UNIQUE INDEX IF NOT EXISTS idx_repository_weight ON recom_repo_scores (repository_id, weight_name);
