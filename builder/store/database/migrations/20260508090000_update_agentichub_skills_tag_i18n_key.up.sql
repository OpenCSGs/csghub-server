SET statement_timeout = 0;

--bun:split

UPDATE tags
SET i18n_key = 'agentichub-skills'
WHERE name = 'agentichub-skills'
  AND category = 'task'
  AND scope = 'skill'
  AND i18n_key IS DISTINCT FROM 'agentichub-skills';
