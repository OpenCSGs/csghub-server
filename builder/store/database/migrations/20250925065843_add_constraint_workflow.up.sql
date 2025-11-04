SET statement_timeout = 0;


--bun:split
WITH duplicate_rows AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY task_id
                              ORDER BY id) AS row_num
    FROM argo_workflows
)
DELETE FROM argo_workflows
WHERE id IN (SELECT id FROM duplicate_rows WHERE row_num > 1);

--bun:split

ALTER TABLE argo_workflows DROP CONSTRAINT IF EXISTS unique_argo_workflow_taskid;

--bun:split

ALTER TABLE argo_workflows ADD CONSTRAINT unique_argo_workflow_taskid UNIQUE (task_id);
