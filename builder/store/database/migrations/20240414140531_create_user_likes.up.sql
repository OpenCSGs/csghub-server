SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_user_likes_userid_repoid;

--bun:split

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_likes_userid_repoid ON user_likes(user_id, repo_id);
