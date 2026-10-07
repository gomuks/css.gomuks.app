-- v0 -> v4 (compatible with v2+): Latest schema
CREATE TABLE theme (
    id          TEXT      PRIMARY KEY,
    name        TEXT      NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_commit INTEGER
);

CREATE TABLE commit (
    theme_id       TEXT,
    version        INTEGER,
    message        TEXT      NOT NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by     TEXT      NOT NULL,
    content        TEXT      NOT NULL,
    description    TEXT      NOT NULL,
    preview_images uuid[]    NOT NULL,

    PRIMARY KEY (theme_id, version),
    CONSTRAINT commit_theme_id_fkey FOREIGN KEY (theme_id) REFERENCES theme (id)
        ON DELETE CASCADE ON UPDATE CASCADE
);
ALTER TABLE theme ADD CONSTRAINT theme_last_commit_fkey FOREIGN KEY (id, last_commit) REFERENCES commit (theme_id, version);

CREATE TABLE preview_image (
    image_id   uuid      PRIMARY KEY,
    theme_id   TEXT      NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT      NOT NULL,
    width      INTEGER   NOT NULL,
    height     INTEGER   NOT NULL,
    mime_type  TEXT      NOT NULL,
    content    bytea     NOT NULL,

    CONSTRAINT preview_image_theme_id_fkey FOREIGN KEY (theme_id) REFERENCES theme (id)
        ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX preview_image_theme_id_idx ON preview_image (theme_id);

CREATE TABLE admin (
    theme_id TEXT,
    user_id  TEXT,

    PRIMARY KEY (theme_id, user_id),
    CONSTRAINT admin_theme_id_fkey FOREIGN KEY (theme_id) REFERENCES theme (id)
        ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX admin_user_id_idx ON admin (user_id);

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
