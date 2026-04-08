SET statement_timeout = 0;

--bun:split

-- Remove unique constraint on repository_id and branch
ALTER TABLE repository_statistics DROP CONSTRAINT IF EXISTS repository_statistics_repository_id_branch_key;

--bun:split

-- Add unique constraint on repository_id
ALTER TABLE repository_statistics ADD CONSTRAINT repository_statistics_repository_id_key UNIQUE (repository_id);

--bun:split

-- Remove branch column
ALTER TABLE repository_statistics DROP COLUMN IF EXISTS branch;
