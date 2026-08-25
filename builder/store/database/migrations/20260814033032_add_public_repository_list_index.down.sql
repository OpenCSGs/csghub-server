SET statement_timeout = 0;

--bun:split

DROP INDEX CONCURRENTLY IF EXISTS idx_recom_total_score_repository;

--bun:split

DROP INDEX CONCURRENTLY IF EXISTS idx_repositories_active_type_stars;

--bun:split

DROP INDEX CONCURRENTLY IF EXISTS idx_repositories_active_type_likes;

--bun:split

DROP INDEX CONCURRENTLY IF EXISTS idx_repositories_active_type_downloads;

--bun:split

DROP INDEX CONCURRENTLY IF EXISTS idx_repositories_active_type_updated;
