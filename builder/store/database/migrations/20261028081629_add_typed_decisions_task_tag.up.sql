SET statement_timeout = 0;

--bun:split

-- typed-decisions is the task the AIGateway reports for decision models such
-- as jev-1.13 and laya. Model task derivation and System One serving already
-- exist; this row is the catalog entry the admin console's task selector reads
-- from GET /admin/system/system_config/tags.
INSERT INTO tags (name, category, "group", scope, built_in, show_name, i18n_key, created_at, updated_at)
VALUES ('typed-decisions', 'task', 'natural_language_processing', 'model', true, '类型化决策', 'typed-decisions', now(), now())
ON CONFLICT (name, category, scope) DO UPDATE
    SET built_in = EXCLUDED.built_in,
        show_name = EXCLUDED.show_name,
        i18n_key = EXCLUDED.i18n_key;
