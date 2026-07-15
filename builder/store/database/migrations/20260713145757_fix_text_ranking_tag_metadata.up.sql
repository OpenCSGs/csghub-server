SET statement_timeout = 0;

--bun:split

UPDATE tags
SET category = 'task',
    "group" = 'natural_language_processing',
    scope = 'model',
    show_name = '文本排序',
    i18n_key = 'text-ranking',
    built_in = TRUE,
    updated_at = CURRENT_TIMESTAMP
WHERE name = 'text-ranking'
  AND scope = 'model';
