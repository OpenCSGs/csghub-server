-- The task category and agent-ui tag are part of the shared seeded tag catalog.
-- The up migration uses ON CONFLICT DO NOTHING, so it cannot distinguish rows
-- it inserted from rows that already existed. Preserve the catalog on rollback.
SELECT 1;
