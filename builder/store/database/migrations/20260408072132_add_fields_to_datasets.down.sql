SET statement_timeout = 0;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS forked;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS price;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS related_dataset_id;

--bun:split

ALTER TABLE datasets DROP COLUMN IF EXISTS dataset_type;
