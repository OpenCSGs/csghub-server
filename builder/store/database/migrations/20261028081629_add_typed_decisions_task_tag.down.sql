SET statement_timeout = 0;

--bun:split

-- repository_tags is a bun m2m join table without a foreign key to tags, so
-- drop the associations first instead of leaving orphan rows behind.
DELETE FROM repository_tags
WHERE tag_id IN (
    SELECT id
    FROM tags
    WHERE name = 'typed-decisions'
        AND category = 'task'
        AND scope = 'model'
);

--bun:split

DELETE FROM tags
WHERE name = 'typed-decisions'
    AND category = 'task'
    AND scope = 'model';
