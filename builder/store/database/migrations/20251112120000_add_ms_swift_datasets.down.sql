SET statement_timeout = 0;

--bun:split

-- Rollback SQL statements for MS-SWIFT dataset tags
DELETE FROM tag_rules WHERE runtime_framework = 'ms-swift' AND repo_type = 'dataset' AND category = 'task' AND source = 'ms';
