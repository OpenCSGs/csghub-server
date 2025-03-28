SET statement_timeout = 0;

--bun:split

-- delete rows where weight_name is not 'total'
DELETE FROM recom_repo_scores WHERE weight_name != 'total';

-- drop column weight_name
ALTER TABLE recom_repo_scores DROP COLUMN IF EXISTS weight_name;

-- drop unique index
DROP INDEX IF EXISTS idx_repository_weight;

-- change pk from id to repository_id 
ALTER TABLE recom_repo_scores DROP COLUMN IF EXISTS id;
ALTER TABLE recom_repo_scores ADD CONSTRAINT recom_repo_scores_pkey PRIMARY KEY (repository_id);

