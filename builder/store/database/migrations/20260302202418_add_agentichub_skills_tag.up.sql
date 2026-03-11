SET statement_timeout = 0;

--bun:split

INSERT INTO tags (name, category, "group", scope, built_in, show_name, i18n_key, created_at, updated_at)
VALUES (
    'agentichub-skills',
    'task',
    'agentichub',
    'skill',
    true,
    'AgenticHub Skills',
    'agentichub-skills',
    NOW(),
    NOW()
)
ON CONFLICT (name, category, scope) DO NOTHING;
