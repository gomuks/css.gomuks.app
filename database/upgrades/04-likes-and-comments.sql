-- v4 (compatible with v2+): Add likes and comments
ALTER TABLE preview_image ALTER COLUMN theme_id SET NOT NULL;

CREATE TABLE "like" (
    theme_id TEXT,
    user_id  TEXT,

    PRIMARY KEY (theme_id, user_id),
    CONSTRAINT like_theme_id_fkey FOREIGN KEY (theme_id) REFERENCES theme (id)
        ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX like_user_id_idx ON "like" (user_id);

CREATE TABLE comment (
    comment_id uuid      PRIMARY KEY,
    theme_id   TEXT      NOT NULL,
    user_id    TEXT      NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    edited_at  TIMESTAMP,
    text       TEXT,
    reply_to   uuid,

    CONSTRAINT comment_theme_id_fkey FOREIGN KEY (theme_id) REFERENCES theme (id)
        ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT comment_reply_to_fkey FOREIGN KEY (reply_to) REFERENCES comment (comment_id)
        ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX comment_theme_id_idx ON comment (theme_id);
