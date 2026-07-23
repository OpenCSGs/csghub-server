SET statement_timeout = 0;

--bun:split

INSERT INTO tags (
    name,
    category,
    "group",
    scope,
    show_name,
    i18n_key,
    built_in
)
SELECT
    'optical-character-recognition',
    'task',
    'computer_vision',
    'model',
    '光学字符识别',
    'optical-character-recognition',
    TRUE
WHERE NOT EXISTS (
    SELECT 1
    FROM tags
    WHERE name = 'optical-character-recognition'
      AND scope = 'model'
);
