SET statement_timeout = 0;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS dataset_type VARCHAR(255) DEFAULT 'normal';

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS related_dataset_id BIGINT;

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS price DECIMAL(10,2);

--bun:split

ALTER TABLE datasets ADD COLUMN IF NOT EXISTS forked BOOLEAN DEFAULT false;
