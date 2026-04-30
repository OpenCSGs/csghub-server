ALTER TABLE repository_tags
ADD COLUMN IF NOT EXISTS source VARCHAR(64) NOT NULL DEFAULT 'auto';

UPDATE repository_tags
SET source = 'auto'
WHERE source IS NULL OR source = '';

UPDATE repository_tags rt
SET source = 'manual'
FROM tags t
WHERE rt.tag_id = t.id
  AND t.category = 'industry';
