-- v0 -> v1 (compatible with v1+): Latest schema
CREATE TABLE theme (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL,
    last_commit INTEGER
);

CREATE TABLE commit (
    theme_id   TEXT,
    version    INTEGER,
    message    TEXT      NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT      NOT NULL,
    content    TEXT      NOT NULL,

    PRIMARY KEY (theme_id, version),
    CONSTRAINT commit_theme_id_fkey FOREIGN KEY (theme_id) REFERENCES theme (id)
        ON DELETE CASCADE ON UPDATE CASCADE
);
ALTER TABLE theme ADD CONSTRAINT theme_last_commit_fkey FOREIGN KEY (id, last_commit) REFERENCES commit (theme_id, version);

CREATE TABLE preview_image (
    image_id   uuid PRIMARY KEY,
    theme_id   TEXT,
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
