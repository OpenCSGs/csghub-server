SET statement_timeout = 0;

--bun:split

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_repositories_active_type_updated
ON repositories (repository_type, updated_at DESC NULLS LAST, id DESC)
WHERE deleted_at IS NULL;

--bun:split

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_repositories_active_type_downloads
ON repositories (repository_type, download_count DESC NULLS LAST, id DESC)
WHERE deleted_at IS NULL;

--bun:split

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_repositories_active_type_likes
ON repositories (repository_type, likes DESC NULLS LAST, id DESC)
WHERE deleted_at IS NULL;

--bun:split

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_repositories_active_type_stars
ON repositories (repository_type, star_count DESC NULLS LAST, id DESC)
WHERE deleted_at IS NULL;

--bun:split

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_recom_total_score_repository
ON recom_repo_scores (score DESC NULLS LAST, repository_id DESC)
WHERE weight_name = 'total';
