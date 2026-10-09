SET statement_timeout = 0;

--bun:split

-- Backfill i18n_key for the audio-driven video tags added by migration
-- 20260720153546_add_audio_driven_video_tags, which used the legacy Tag
-- struct without an I18nKey field. The component layer resolves the
-- display name via "Tag.I18nKey.<i18n_key>", so a missing i18n_key leaves
-- show_name empty and the UI falls back to the raw English tag name.
UPDATE tags
SET i18n_key = 'audio-text-to-video'
WHERE name = 'audio-text-to-video'
  AND category = 'task'
  AND scope = 'model'
  AND (i18n_key IS NULL OR i18n_key = '');

--bun:split

UPDATE tags
SET i18n_key = 'audio-image-text-to-video'
WHERE name = 'audio-image-text-to-video'
  AND category = 'task'
  AND scope = 'model'
  AND (i18n_key IS NULL OR i18n_key = '');

--bun:split

UPDATE tags
SET i18n_key = 'audio-driven-video-continuation'
WHERE name = 'audio-driven-video-continuation'
  AND category = 'task'
  AND scope = 'model'
  AND (i18n_key IS NULL OR i18n_key = '');

--bun:split

-- Backfill show_name for the resource and runtime_framework tag categories
-- added by migration 20231212064618_initialize_tag_categories, whose seed
-- entries omitted show_name. The handler resolves the display name via
-- "tag_category.name.<show_name>", so an empty show_name makes the UI fall
-- back to the raw English category name. Setting show_name to the category
-- name matches the existing tag_category.name.<name> translation keys.
UPDATE tag_categories
SET show_name = 'resource'
WHERE name = 'resource'
  AND scope = 'model'
  AND (show_name IS NULL OR show_name = '');

--bun:split

UPDATE tag_categories
SET show_name = 'runtime_framework'
WHERE name = 'runtime_framework'
  AND scope = 'model'
  AND (show_name IS NULL OR show_name = '');
