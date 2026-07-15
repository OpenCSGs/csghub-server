SET statement_timeout = 0;

--bun:split

ALTER TABLE mirror_tasks
    ADD COLUMN IF NOT EXISTS repo_job_id BIGINT,
    ADD COLUMN IF NOT EXISTS lfs_job_id BIGINT;
