SET statement_timeout = 0;

--bun:split
CREATE INDEX IF NOT EXISTS idx_recom_total_weight_score
ON recom_repo_scores (repository_id, score DESC)
WHERE weight_name = 'total';

--bun:split
