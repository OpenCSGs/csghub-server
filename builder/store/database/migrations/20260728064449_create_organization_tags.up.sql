SET statement_timeout = 0;

--bun:split

CREATE TABLE IF NOT EXISTS organization_tags (
    id SERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, tag_id)
);

--bun:split

CREATE INDEX IF NOT EXISTS idx_organization_tags_organization_id ON organization_tags(organization_id);

--bun:split

CREATE INDEX IF NOT EXISTS idx_organization_tags_tag_id ON organization_tags(tag_id);
