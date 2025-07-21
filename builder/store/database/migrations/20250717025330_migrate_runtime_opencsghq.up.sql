SET statement_timeout = 0;

--bun:split

-- migrate all image to opencsghq namespace since csghub 2.0
DELETE FROM runtime_frameworks WHERE frame_image NOT LIKE '%/%';

--bun:split

DELETE FROM runtime_architectures WHERE runtime_framework_id NOT IN (SELECT id FROM runtime_frameworks);
