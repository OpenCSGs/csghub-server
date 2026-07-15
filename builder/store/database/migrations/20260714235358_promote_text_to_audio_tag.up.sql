SET statement_timeout = 0;

--bun:split

UPDATE tags
SET category = 'task',
    "group" = 'audio_processing',
    scope = 'model',
    show_name = '文本转音频',
    i18n_key = 'text-to-audio',
    built_in = TRUE,
    updated_at = CURRENT_TIMESTAMP
WHERE name = 'text-to-audio'
  AND scope = 'model';

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
    'text-to-audio',
    'task',
    'audio_processing',
    'model',
    '文本转音频',
    'text-to-audio',
    TRUE
WHERE NOT EXISTS (
    SELECT 1
    FROM tags
    WHERE name = 'text-to-audio'
      AND scope = 'model'
);

--bun:split

INSERT INTO repository_tags (repository_id, tag_id)
SELECT DISTINCT
    repository.id,
    text_to_audio.id
FROM repositories AS repository
JOIN repository_tags AS existing_repository_tag
  ON existing_repository_tag.repository_id = repository.id
JOIN tags AS existing_tag
  ON existing_tag.id = existing_repository_tag.tag_id
JOIN tags AS text_to_audio
  ON text_to_audio.name = 'text-to-audio'
 AND text_to_audio.scope = 'model'
WHERE existing_tag.name = 'text-to-speech'
  AND existing_tag.scope = 'model'
  AND (
      LOWER(repository.name) = 'audiofly'
      OR LOWER(COALESCE(
          NULLIF(repository.hf_path, ''),
          NULLIF(repository.ms_path, ''),
          NULLIF(repository.csg_path, ''),
          repository.path
      )) LIKE '%/audiofly'
  )
ON CONFLICT (repository_id, tag_id) DO NOTHING;
