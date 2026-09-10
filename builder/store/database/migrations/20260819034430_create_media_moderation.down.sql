SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_comment_media_comment_id;

--bun:split

DROP TABLE IF EXISTS comment_media;

--bun:split

DROP INDEX IF EXISTS idx_media_moderations_task_id;

--bun:split

DROP TABLE IF EXISTS media_moderations;
