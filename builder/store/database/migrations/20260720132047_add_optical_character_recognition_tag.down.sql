SET statement_timeout = 0;

--bun:split

DELETE FROM tags
WHERE name = 'optical-character-recognition'
  AND scope = 'model'
  AND built_in = TRUE;
