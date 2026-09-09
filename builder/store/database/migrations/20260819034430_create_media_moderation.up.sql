SET statement_timeout = 0;

--bun:split

CREATE TABLE IF NOT EXISTS media_moderations (
    id              BIGSERIAL PRIMARY KEY,
    resource_key    TEXT NOT NULL UNIQUE,
    data_id         TEXT NOT NULL UNIQUE,
    seed            TEXT NOT NULL,
    task_id         TEXT NOT NULL DEFAULT '',
    media_type      TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    reason          TEXT NOT NULL DEFAULT '',
    last_attempt_at TIMESTAMPTZ,
    submitted_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT media_moderations_status_chk CHECK (status IN ('pending','pass','reject','error')),
    CONSTRAINT media_moderations_media_type_chk CHECK (media_type IN ('image','audio','video'))
);

--bun:split

CREATE INDEX IF NOT EXISTS idx_media_moderations_task_id ON media_moderations(task_id) WHERE task_id <> '';

--bun:split

CREATE TABLE IF NOT EXISTS comment_media (
    comment_id BIGINT NOT NULL,
    data_id    TEXT NOT NULL,
    media_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (comment_id, data_id),
    CONSTRAINT comment_media_media_type_chk CHECK (media_type IN ('image','audio','video'))
);

--bun:split

CREATE INDEX IF NOT EXISTS idx_comment_media_comment_id ON comment_media(comment_id);
