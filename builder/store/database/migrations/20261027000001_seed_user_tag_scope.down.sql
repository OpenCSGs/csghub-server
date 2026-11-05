SET statement_timeout = 0;

--bun:split

DROP INDEX IF EXISTS idx_tag_id_link_user_id;

--bun:split

-- Remove the user_tags associations first: they have no FK to tags, so
-- deleting the tags below would leave orphan rows behind.
DELETE FROM user_tags
WHERE tag_id IN (
    SELECT routing_tag.id
    FROM tags AS routing_tag
    WHERE routing_tag.scope = 'user'
        AND routing_tag.category = 'user'
        AND (routing_tag.name = 'low_priority' OR routing_tag.name LIKE 'priority:%')
);

--bun:split

-- Built-in low_priority plus the operator-created priority:<n> tiers.
DELETE FROM tags
WHERE scope = 'user'
    AND category = 'user'
    AND (name = 'low_priority' OR name LIKE 'priority:%');

--bun:split

DELETE FROM tag_categories WHERE name = 'user' AND scope = 'user';
