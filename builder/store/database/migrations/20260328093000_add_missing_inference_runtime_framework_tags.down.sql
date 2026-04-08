SET statement_timeout = 0;

--bun:split

DELETE FROM tags
WHERE category = 'runtime_framework'
  AND scope = 'model'
  AND "group" = 'inference'
  AND name IN ('hf-inference-toolkit', 'lightx2v', 'audio-fish');
