-- v2: Move description from theme to commit
ALTER TABLE commit ADD COLUMN description TEXT NOT NULL DEFAULT '';
UPDATE commit SET description=(SELECT description FROM theme WHERE theme.id = commit.theme_id);
ALTER TABLE theme DROP COLUMN description;
ALTER TABLE commit ALTER COLUMN description SET NOT NULL;

ALTER TABLE commit ADD COLUMN preview_images uuid[] NOT NULL DEFAULT '{}';
UPDATE commit SET preview_images=COALESCE((
    SELECT array_agg(image_id) FROM preview_image WHERE preview_image.theme_id = commit.theme_id
), '{}'::uuid[]);
ALTER TABLE commit ALTER COLUMN preview_images SET NOT NULL;
