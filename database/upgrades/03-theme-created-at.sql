-- v3 (compatible with v2+): Add created at time for themes
ALTER TABLE theme ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;
UPDATE theme SET created_at=(SELECT created_at FROM commit WHERE commit.theme_id=theme.id AND commit.version=1);
