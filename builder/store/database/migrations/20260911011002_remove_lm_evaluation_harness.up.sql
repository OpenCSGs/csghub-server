SET statement_timeout = 0;

--bun:split

-- lm-evaluation-harness is retired. Its engine config is removed from
-- configs/evaluation, but that only stops the scan from re-creating the framework;
-- rows already in the database have to be removed here.

-- Links from repositories to the retired framework, which would otherwise be left
-- pointing at a framework that no longer exists.
DELETE FROM repositories_runtime_frameworks
WHERE runtime_framework_id IN (
    SELECT id FROM runtime_frameworks WHERE frame_name = 'lm-evaluation-harness'
);

--bun:split

DELETE FROM runtime_architectures
WHERE runtime_framework_id IN (
    SELECT id FROM runtime_frameworks WHERE frame_name = 'lm-evaluation-harness'
);

--bun:split

DELETE FROM runtime_frameworks WHERE frame_name = 'lm-evaluation-harness';

--bun:split

DELETE FROM tag_rules WHERE runtime_framework = 'lm-evaluation-harness';

--bun:split

DELETE FROM tags WHERE name = 'lm-evaluation-harness' AND category = 'runtime_framework';
