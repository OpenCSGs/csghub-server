SET statement_timeout = 0;

--bun:split

ALTER TABLE mirror_tasks
    DROP COLUMN IF EXISTS lfs_job_id,
    DROP COLUMN IF EXISTS repo_job_id;
